package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	stdsync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

func TestHoldFor(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	ep := func(mut func(*models.SonarrEpisode)) models.SonarrEpisode {
		e := models.SonarrEpisode{SeasonNumber: 1, Monitored: true, AirDateUTC: now.Add(-24 * time.Hour)}
		if mut != nil {
			mut(&e)
		}
		return e
	}
	for name, tc := range map[string]struct {
		ep     models.SonarrEpisode
		heldAt time.Time
		isHeld bool
		hold   time.Duration
		want   holdStep
	}{
		"aired yesterday":         {ep: ep(nil), hold: week, want: holdStart},
		"airs next week":          {ep: ep(func(e *models.SonarrEpisode) { e.AirDateUTC = now.Add(week) }), hold: week, want: holdStart},
		"no air date":             {ep: ep(func(e *models.SonarrEpisode) { e.AirDateUTC = time.Time{} }), hold: week, want: holdStart},
		"aired before the period": {ep: ep(func(e *models.SonarrEpisode) { e.AirDateUTC = now.Add(-2 * week) }), hold: week, want: holdNone},
		"already downloaded":      {ep: ep(func(e *models.SonarrEpisode) { e.HasFile = true }), hold: week, want: holdNone},
		"special":                 {ep: ep(func(e *models.SonarrEpisode) { e.SeasonNumber = 0 }), hold: week, want: holdNone},
		"holding off":             {ep: ep(nil), hold: 0, want: holdNone},
		"still waiting":           {ep: ep(func(e *models.SonarrEpisode) { e.Monitored = false }), heldAt: now.Add(-24 * time.Hour), isHeld: true, hold: week, want: holdKeep},
		"waited long enough":      {ep: ep(func(e *models.SonarrEpisode) { e.Monitored = false }), heldAt: now.Add(-week), isHeld: true, hold: week, want: holdRelease},
		"holding turned off":      {ep: ep(func(e *models.SonarrEpisode) { e.Monitored = false }), heldAt: now, isHeld: true, hold: 0, want: holdRelease},
		"monitored again by hand": {ep: ep(nil), heldAt: now, isHeld: true, hold: week, want: holdRelease},
	} {
		assert.Equal(t, tc.want, holdFor(tc.ep, tc.heldAt, tc.isHeld, tc.hold, now), name)
	}
}

const holdAFLRow = `<tr><td class="Number">%d</td><td class="Title"><a href="#">Episode %d</a></td><td class="Type"><span>%s</span></td></tr>`

// holdFixture is a show with two aired episodes: AnimeFillerList lists
// episode 1 from the start and episode 2 only once listSecond is set.
type holdFixture struct {
	mu         stdsync.Mutex
	monitored  map[int]bool
	listSecond string // episode 2's type once listed
}

func (f *holdFixture) sonarr(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	aired := time.Now().Add(-24 * time.Hour)
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series/100":
		_ = json.NewEncoder(w).Encode(models.SonarrSeries{ID: 100, Title: "Test Show"})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/episode":
		eps := make([]models.SonarrEpisode, 0, 2)
		for n := 1; n <= 2; n++ {
			abs := n
			eps = append(eps, models.SonarrEpisode{ID: 9000 + n, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: &abs, Title: "Episode", Monitored: f.monitored[9000+n], AirDateUTC: aired})
		}
		_ = json.NewEncoder(w).Encode(eps)
	case r.Method == http.MethodPut && r.URL.Path == "/api/v3/episode/monitor":
		var body struct {
			EpisodeIDs []int `json:"episodeIds"`
			Monitored  bool  `json:"monitored"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, id := range body.EpisodeIDs {
			f.monitored[id] = body.Monitored
		}
	default:
		http.NotFound(w, r)
	}
}

func (f *holdFixture) afl(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows := fmt.Sprintf(holdAFLRow, 1, 1, "canon")
	if f.listSecond != "" {
		rows += fmt.Sprintf(holdAFLRow, 2, 2, f.listSecond)
	}
	_, _ = io.WriteString(w, `<html><body><span property="dc:title" content="Test Show"></span><table class="EpisodeList"><tbody>`+rows+`</tbody></table></body></html>`)
}

func newHoldEngine(t *testing.T, hold string) (*Engine, *holdFixture, int64) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	f := &holdFixture{monitored: map[int]bool{9001: true, 9002: true}}
	sonarrSrv := httptest.NewServer(http.HandlerFunc(f.sonarr))
	t.Cleanup(sonarrSrv.Close)
	aflSrv := httptest.NewServer(http.HandlerFunc(f.afl))
	t.Cleanup(aflSrv.Close)

	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarrSrv.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 5 * time.Second
	cfg.HoldUnclassified = hold
	e := New(cfg, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)

	m := &models.ShowMapping{
		SonarrSeriesID: 100, SonarrTitle: "Test Show", AFLSlug: "test-show", AFLTitle: "Test Show", Enabled: true,
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor, AnimeCanonAction: models.ActionMonitor,
		StartMode: models.StartModeFromBeginning, WatermarkMode: models.WatermarkModeManual,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	return e, f, m.ID
}

// resync runs one sync of the show, as the scheduled sync would.
func resync(t *testing.T, e *Engine, id int64) processSeriesResult {
	t.Helper()
	ctx := context.Background()
	mapping, err := e.db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	res, err := e.processSeries(ctx, models.SonarrSeries{ID: 100, Title: "Test Show"}, mapping, false, map[int64]int64{id: e.CaptureMappingEpoch(id)})
	require.NoError(t, err)
	return res
}

func heldIDs(t *testing.T, e *Engine, id int64) []int {
	t.Helper()
	held, err := e.db.GetHeldEpisodes(context.Background(), id)
	require.NoError(t, err)
	ids := make([]int, 0, len(held))
	for k := range held {
		ids = append(ids, k)
	}
	return ids
}

func TestUnclassifiedEpisodeIsHeldUntilListed(t *testing.T) {
	e, f, id := newHoldEngine(t, "7d")

	res := resync(t, e, id)
	assert.False(t, f.monitored[9002], "an unlisted new episode waits unmonitored")
	assert.True(t, f.monitored[9001])
	assert.Equal(t, []int{9002}, heldIDs(t, e, id))
	assert.Equal(t, 1, res.HeldCount)
	assert.Contains(t, summarizeProcessResult(res), "holding 1 for AnimeFillerList")

	// Still unlisted: stays held without another Sonarr write.
	res = resync(t, e, id)
	assert.False(t, f.monitored[9002])
	assert.Equal(t, 1, res.HeldCount)
	assert.Equal(t, 0, res.EpisodesUpdated)

	// Listed as canon: the rules monitor it and the hold is dropped.
	f.listSecond = "canon"
	resync(t, e, id)
	assert.True(t, f.monitored[9002])
	assert.Empty(t, heldIDs(t, e, id))
}

func TestHeldFillerStaysUnmonitoredOnceListed(t *testing.T) {
	e, f, id := newHoldEngine(t, "7d")
	resync(t, e, id)
	f.listSecond = "filler"
	resync(t, e, id)
	assert.False(t, f.monitored[9002])
	assert.Empty(t, heldIDs(t, e, id))
}

func TestTurningHoldOffReleasesHeldEpisodes(t *testing.T) {
	e, f, id := newHoldEngine(t, "7d")
	resync(t, e, id)
	require.False(t, f.monitored[9002])

	e.cfg.Lock()
	e.cfg.HoldUnclassified = "off"
	e.cfg.Unlock()
	resync(t, e, id)
	assert.True(t, f.monitored[9002])
	assert.Empty(t, heldIDs(t, e, id))
}

// With holding off, an unlisted episode is monitored like before.
func TestHoldOffMonitorsUnlistedEpisodes(t *testing.T) {
	e, f, id := newHoldEngine(t, "off")
	f.monitored[9002] = false
	resync(t, e, id)
	assert.True(t, f.monitored[9002])
	assert.Empty(t, heldIDs(t, e, id))
}
