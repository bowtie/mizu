package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"
)

// syncRaceAFLEpisodeHTML is the Anime Filler List episode page shape the
// scraper parses: a dc:title span and an EpisodeList table with one episode.
const syncRaceAFLEpisodeHTML = `<!DOCTYPE html><html><body>
<span property="dc:title" content="Test Show"></span>
<table class="EpisodeList"><tbody>
<tr><td class="Number">1</td><td class="Title"><a href="#">Episode 1</a></td><td class="Type"><span>filler</span></td></tr>
</tbody></table>
</body></html>`

// syncRaceSonarr is a minimal Sonarr v3 API stub. Episode monitored state is
// mutable so a user toggle and a sync bulk-apply are both observable. With
// blockEpisodesFirst it parks the first episode GET (the sync's) so a user
// write can land mid-fetch; later episode GETs (e.g. a hydrate's) serve
// immediately, mirroring the AFL seam's blockFirst pattern.
type syncRaceSonarr struct {
	mu                 sync.Mutex
	series             models.SonarrSeries
	episodes           []models.SonarrEpisode
	monitored          map[int]bool
	blockEpisodesFirst bool
	episodesBlocked    bool
	episodesEntered    chan struct{}
	episodesRelease    chan struct{}
}

func (f *syncRaceSonarr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/v3/episode" && f.blockEpisodesFirst {
		f.mu.Lock()
		shouldBlock := !f.episodesBlocked
		if shouldBlock {
			f.episodesBlocked = true
		}
		f.mu.Unlock()
		if shouldBlock {
			close(f.episodesEntered)
			<-f.episodesRelease
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v3/series/"):
		writeJSON(w, http.StatusOK, f.series)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/episode":
		eps := make([]models.SonarrEpisode, 0, len(f.episodes))
		for _, ep := range f.episodes {
			ep.Monitored = f.monitored[ep.ID]
			eps = append(eps, ep)
		}
		writeJSON(w, http.StatusOK, eps)
	case r.Method == http.MethodPut && r.URL.Path == "/api/v3/episode/monitor":
		var body struct {
			EpisodeIDs []int `json:"episodeIds"`
			Monitored  bool  `json:"monitored"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, id := range body.EpisodeIDs {
			f.monitored[id] = body.Monitored
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func (f *syncRaceSonarr) monitoredState(episodeID int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.monitored[episodeID]
}

// syncRaceAFL is an Anime Filler List stub. With blockFirst it parks the
// sync's first episode fetch (so a user write can land mid-run) and serves
// subsequent requests (e.g. a hydrate's fetch) immediately.
type syncRaceAFL struct {
	mu         sync.Mutex
	blockFirst bool
	blocked    bool
	entered    chan struct{}
	release    chan struct{}
	html       string
}

func (f *syncRaceAFL) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	shouldBlock := f.blockFirst && !f.blocked
	if shouldBlock {
		f.blocked = true
	}
	f.mu.Unlock()

	if shouldBlock {
		close(f.entered)
		<-f.release
	}

	w.Header().Set("Content-Type", "text/html")
	_, _ = io.WriteString(w, f.html)
}

// newSyncRaceEngine builds a sync engine pointed at a fake Sonarr server,
// with the AFL scraper redirected to a local stub and no rate limiting.
func newSyncRaceEngine(t *testing.T, sonarrURL string, db *database.DB) *syncengine.Engine {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarrURL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 5 * time.Second
	cfg.NtfyEnabled = false
	return syncengine.New(cfg, db)
}

func insertSyncRaceMapping(t *testing.T, db *database.DB, watermarkMode models.WatermarkMode) int64 {
	t.Helper()
	m := &models.ShowMapping{
		SonarrSeriesID:   100,
		SonarrTitle:      "Test Show",
		AFLSlug:          "test-show",
		AFLTitle:         "Test Show", // set so the sync's legacy backfill never fires
		Enabled:          true,
		FillerAction:     models.ActionUnmonitor,
		MixedAction:      models.ActionUnmonitor,
		AnimeCanonAction: models.ActionUnmonitor,
		StartMode:        models.StartModeCurrentProgress,
		WatermarkMode:    watermarkMode,
		SyncWatermark:    0,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	require.NotZero(t, m.ID, "UpsertMapping should populate the mapping ID")
	return m.ID
}

// requestWithChiParams attaches chi URL parameters the handlers read via
// chi.URLParam, as chi.Router does in production.
func requestWithChiParams(method, target string, params map[string]string, body string) *http.Request {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, r)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Keys = make([]string, 0, len(params))
	rctx.URLParams.Values = make([]string, 0, len(params))
	for k, v := range params {
		rctx.URLParams.Keys = append(rctx.URLParams.Keys, k)
		rctx.URLParams.Values = append(rctx.URLParams.Values, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func runSyncCollectErr(t *testing.T, e *syncengine.Engine, ctx context.Context) <-chan error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- e.RunSync(ctx, "manual") }()
	return errCh
}

// TestSyncDoesNotClobberMidSyncEpisodeToggle is the failing-first reproduction
// of the strongest audit finding: a user monitor toggle landing while a sync
// is in flight must not be reverted by the sync's stale write-backs — in
// Sonarr or in the DB.
func TestSyncDoesNotClobberMidSyncEpisodeToggle(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	abs := 1
	sonarr := &syncRaceSonarr{
		series: models.SonarrSeries{ID: 100, Title: "Test Show", Overview: "Synopsis"},
		episodes: []models.SonarrEpisode{{
			ID:                    9001,
			SeriesID:              100,
			SeasonNumber:          1,
			EpisodeNumber:         1,
			AbsoluteEpisodeNumber: &abs,
			Title:                 "Episode 1",
			Monitored:             false,
			HasFile:               false,
			AirDateUTC:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}},
		monitored: map[int]bool{9001: false},
	}
	sonarrSrv := httptest.NewServer(sonarr)
	defer sonarrSrv.Close()

	afl := &syncRaceAFL{blockFirst: true, entered: make(chan struct{}), release: make(chan struct{}), html: syncRaceAFLEpisodeHTML}
	aflSrv := httptest.NewServer(afl)
	defer aflSrv.Close()

	e := newSyncRaceEngine(t, sonarrSrv.URL, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)
	id := insertSyncRaceMapping(t, db, models.WatermarkModeManual)

	// Pre-seed the episode row so both the user's toggle and the sync's
	// replace are observable in the DB.
	absInt := 1
	past := models.DateOnly(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, db.ReplaceEpisodesForMapping(ctx, id, []models.EpisodeInfo{{
		MappingID:             id,
		EpisodeNum:            1,
		Title:                 "Episode 1",
		Type:                  models.EpisodeFiller,
		SonarrEpisodeID:       9001,
		SeasonNumber:          1,
		SonarrEpisodeNumber:   1,
		AbsoluteEpisodeNumber: &absInt,
		AirDate:               &past,
		Monitored:             false,
	}}))

	syncErr := runSyncCollectErr(t, e, ctx)
	<-afl.entered // sync paused mid-run, blocked in the AFL fetch

	// User toggles episode 9001 to monitored while the sync is in flight.
	rec := httptest.NewRecorder()
	handleSetEpisodeMonitor(db, e, newEpisodeCache(2*time.Minute))(
		rec,
		requestWithChiParams(http.MethodPut, fmt.Sprintf("/api/mappings/%d/episodes/9001/monitor", id),
			map[string]string{"id": fmt.Sprint(id), "epId": "9001"}, `{"monitored":true}`),
	)
	require.Equal(t, http.StatusOK, rec.Code)

	close(afl.release)
	select {
	case err := <-syncErr:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for sync to finish")
	}

	// The user's toggle must survive in Sonarr...
	assert.True(t, sonarr.monitoredState(9001), "sync must not revert the user's toggle in Sonarr")
	// ...and in the DB.
	eps, err := db.GetEpisodes(ctx, id)
	require.NoError(t, err)
	require.Len(t, eps, 1)
	assert.True(t, eps[0].Monitored, "sync must not revert the user's toggle in the DB")

	// The sync log records the mapping as skipped rather than processed.
	latest, err := db.GetLatestSyncLog(ctx)
	require.NoError(t, err)
	items, err := db.GetSyncLogItems(ctx, []int64{latest.ID})
	require.NoError(t, err)
	require.Len(t, items[latest.ID], 1)
	assert.Equal(t, "skipped", items[latest.ID][0].Status)
}

// TestSyncDoesNotClobberMidSyncWatermarkEdit is the failing-first reproduction
// for settings edits: a manual watermark set while a sync is in flight must
// survive, instead of the sync persisting its stale auto-computed value over it.
func TestSyncDoesNotClobberMidSyncWatermarkEdit(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	abs := 1
	sonarr := &syncRaceSonarr{
		series: models.SonarrSeries{ID: 100, Title: "Test Show", Overview: "Synopsis"},
		episodes: []models.SonarrEpisode{{
			ID:                    9001,
			SeriesID:              100,
			SeasonNumber:          1,
			EpisodeNumber:         1,
			AbsoluteEpisodeNumber: &abs,
			Title:                 "Episode 1",
			Monitored:             false,
			HasFile:               true, // drives the auto watermark computation
			AirDateUTC:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}},
		monitored: map[int]bool{9001: false},
	}
	sonarrSrv := httptest.NewServer(sonarr)
	defer sonarrSrv.Close()

	afl := &syncRaceAFL{blockFirst: true, entered: make(chan struct{}), release: make(chan struct{}), html: syncRaceAFLEpisodeHTML}
	aflSrv := httptest.NewServer(afl)
	defer aflSrv.Close()

	e := newSyncRaceEngine(t, sonarrSrv.URL, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)
	id := insertSyncRaceMapping(t, db, models.WatermarkModeAuto)

	syncErr := runSyncCollectErr(t, e, ctx)
	<-afl.entered // sync paused mid-run, blocked in the AFL fetch

	// User sets a manual watermark while the sync is in flight.
	rec := httptest.NewRecorder()
	handleUpdateMapping(e, db)(rec, requestWithChiID(http.MethodPut, fmt.Sprintf("/api/mappings/%d", id), fmt.Sprint(id), `{"sync_watermark":42}`))
	require.Equal(t, http.StatusOK, rec.Code)

	close(afl.release)
	select {
	case err := <-syncErr:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for sync to finish")
	}

	// The user's manual watermark must survive; the sync must not persist
	// its stale auto-computed value (1) and mode over it.
	current, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, 42, current.SyncWatermark, "sync must not clobber the user's manual watermark")
	assert.Equal(t, models.WatermarkModeManual, current.WatermarkMode, "sync must not clobber the user's watermark mode")
}

// TestSyncDoesNotClobberEditDuringEpisodeFetch covers the guard between the
// Sonarr episode fetch and the watermark write-back: a manual watermark set
// while the sync is parked in GetEpisodes must survive, instead of the sync
// persisting its stale auto-computed value over it.
func TestSyncDoesNotClobberEditDuringEpisodeFetch(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	abs := 1
	sonarr := &syncRaceSonarr{
		series: models.SonarrSeries{ID: 100, Title: "Test Show", Overview: "Synopsis"},
		episodes: []models.SonarrEpisode{{
			ID:                    9001,
			SeriesID:              100,
			SeasonNumber:          1,
			EpisodeNumber:         1,
			AbsoluteEpisodeNumber: &abs,
			Title:                 "Episode 1",
			Monitored:             false,
			HasFile:               true, // drives the auto watermark computation
			AirDateUTC:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}},
		monitored:          map[int]bool{9001: false},
		blockEpisodesFirst: true,
		episodesEntered:    make(chan struct{}),
		episodesRelease:    make(chan struct{}),
	}
	sonarrSrv := httptest.NewServer(sonarr)
	defer sonarrSrv.Close()

	afl := &syncRaceAFL{html: syncRaceAFLEpisodeHTML}
	aflSrv := httptest.NewServer(afl)
	defer aflSrv.Close()

	e := newSyncRaceEngine(t, sonarrSrv.URL, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)
	id := insertSyncRaceMapping(t, db, models.WatermarkModeAuto)

	syncErr := runSyncCollectErr(t, e, ctx)
	<-sonarr.episodesEntered // sync parked in the Sonarr episode fetch

	// User sets a manual watermark while the sync is mid-GetEpisodes. The
	// handler's own hydrate re-fetches episodes (served immediately — only the
	// sync's first episode GET is parked), so it completes without the release.
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		rec := httptest.NewRecorder()
		handleUpdateMapping(e, db)(rec, requestWithChiID(http.MethodPut, fmt.Sprintf("/api/mappings/%d", id), fmt.Sprint(id), `{"sync_watermark":42}`))
		require.Equal(t, http.StatusOK, rec.Code)
	}()
	select {
	case <-handlerDone:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the user edit handler")
	}

	close(sonarr.episodesRelease)
	select {
	case err := <-syncErr:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for sync to finish")
	}

	// The user's manual watermark must survive; the sync must not persist
	// its stale auto-computed value and mode over it.
	current, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, 42, current.SyncWatermark, "sync must not clobber the user's manual watermark set during the episode fetch")
	assert.Equal(t, models.WatermarkModeManual, current.WatermarkMode, "sync must not clobber the user's watermark mode set during the episode fetch")

	// The sync log records the mapping as skipped rather than processed.
	latest, err := db.GetLatestSyncLog(ctx)
	require.NoError(t, err)
	items, err := db.GetSyncLogItems(ctx, []int64{latest.ID})
	require.NoError(t, err)
	require.Len(t, items[latest.ID], 1)
	assert.Equal(t, "skipped", items[latest.ID][0].Status)
}

// TestSyncProcessesUnmodifiedMappingNormally is the control: a mapping with no
// user writes mid-sync is processed fully — the epoch guard must not over-skip.
func TestSyncProcessesUnmodifiedMappingNormally(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	abs := 1
	sonarr := &syncRaceSonarr{
		series: models.SonarrSeries{ID: 100, Title: "Test Show", Overview: "Synopsis"},
		episodes: []models.SonarrEpisode{{
			ID:                    9001,
			SeriesID:              100,
			SeasonNumber:          1,
			EpisodeNumber:         1,
			AbsoluteEpisodeNumber: &abs,
			Title:                 "Episode 1",
			Monitored:             false,
			HasFile:               false,
			AirDateUTC:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}},
		monitored: map[int]bool{9001: false},
	}
	sonarrSrv := httptest.NewServer(sonarr)
	defer sonarrSrv.Close()

	afl := &syncRaceAFL{blockFirst: false, html: syncRaceAFLEpisodeHTML}
	aflSrv := httptest.NewServer(afl)
	defer aflSrv.Close()

	e := newSyncRaceEngine(t, sonarrSrv.URL, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)
	id := insertSyncRaceMapping(t, db, models.WatermarkModeManual)

	require.NoError(t, e.RunSync(ctx, "manual"))

	// The sync processed the mapping: episodes written, no skipped item.
	eps, err := db.GetEpisodes(ctx, id)
	require.NoError(t, err)
	require.Len(t, eps, 1, "unmodified mapping should have its episodes written by the sync")
	assert.Equal(t, 9001, eps[0].SonarrEpisodeID)

	latest, err := db.GetLatestSyncLog(ctx)
	require.NoError(t, err)
	items, err := db.GetSyncLogItems(ctx, []int64{latest.ID})
	require.NoError(t, err)
	require.Len(t, items[latest.ID], 1)
	assert.Equal(t, "success", items[latest.ID][0].Status, "unmodified mapping must not be skipped")
}
