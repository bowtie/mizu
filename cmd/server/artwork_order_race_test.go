package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

// blockOnceArtworkProvider parks only the FIRST fetch — the refresh-all's
// fetch for the first mapping it reaches — and serves every subsequent fetch
// immediately (mirroring the syncRaceAFL blockFirst pattern). This lets a
// per-show refresh or source switch for a later mapping complete while the
// refresh-all is parked, so the test can reproduce the older-fetch-lands-last
// ordering deterministically.
type blockOnceArtworkProvider struct {
	mu        sync.Mutex
	blocked   bool
	entered   chan struct{}
	release   chan struct{}
	posterURL string
}

func (p *blockOnceArtworkProvider) Name() string    { return "anilist" }
func (p *blockOnceArtworkProvider) Available() bool { return true }
func (p *blockOnceArtworkProvider) Fetch(ctx context.Context, _ models.SonarrSeries) (*artwork.Artwork, error) {
	p.mu.Lock()
	shouldBlock := !p.blocked
	if shouldBlock {
		p.blocked = true
	}
	p.mu.Unlock()
	if shouldBlock {
		close(p.entered)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.release:
		}
	}
	return &artwork.Artwork{PosterURL: p.posterURL, Source: "anilist"}, nil
}

// namedStaticArtworkProvider returns artwork immediately under an arbitrary
// provider name, so two providers can serve distinguishable images.
type namedStaticArtworkProvider struct {
	name      string
	posterURL string
}

func (p *namedStaticArtworkProvider) Name() string    { return p.name }
func (p *namedStaticArtworkProvider) Available() bool { return true }
func (p *namedStaticArtworkProvider) Fetch(context.Context, models.SonarrSeries) (*artwork.Artwork, error) {
	return &artwork.Artwork{PosterURL: p.posterURL, Source: p.name}, nil
}

// artworkImageServer serves a distinguishable JPEG body so tests can tell
// which provider's download won.
func artworkImageServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(body))
	}))
}

// newOrderingRaceManager builds a manager whose refresh-all parks on the
// first mapping's fetch ("anilist" provider) while a second, static "kitsu"
// provider serves immediate fetches.
func newOrderingRaceManager(t *testing.T, db *database.DB) (mgr *artwork.Manager, artDir string, anilist *blockOnceArtworkProvider) {
	t.Helper()
	anilistSrv := artworkImageServer(t, "anilist-jpeg")
	t.Cleanup(anilistSrv.Close)
	kitsuSrv := artworkImageServer(t, "kitsu-jpeg")
	t.Cleanup(kitsuSrv.Close)

	artDir = filepath.Join(t.TempDir(), "artwork")
	mgr = artwork.NewManager(artDir, db)
	anilist = &blockOnceArtworkProvider{
		entered:   make(chan struct{}),
		release:   make(chan struct{}),
		posterURL: anilistSrv.URL + "/poster.jpg",
	}
	mgr.Register(anilist)
	mgr.Register(&namedStaticArtworkProvider{name: "kitsu", posterURL: kitsuSrv.URL + "/poster.jpg"})
	return mgr, artDir, anilist
}

func readArtworkPoster(t *testing.T, artDir string, mappingID int64) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(artDir, fmt.Sprintf("%d", mappingID), "poster.jpg"))
	require.NoError(t, err, "mapping %d should have a downloaded poster", mappingID)
	return string(data)
}

// TestRefreshAllDoesNotClobberNewerPerShowRefresh is the failing-first
// reproduction of the ordering race: the user refreshes one show while a
// refresh-all is in flight, and the refresh-all's older fetch for that show
// lands last. The per-show refresh's artwork must win — the refresh-all must
// skip its stale write-back instead of overwriting the later choice.
func TestRefreshAllDoesNotClobberNewerPerShowRefresh(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir, anilist := newOrderingRaceManager(t, db)
	ctx := context.Background()

	// A parks the refresh-all (its fetch is the first provider call); X has
	// no override, so the refresh-all resolves it to the global default
	// "anilist" — the stale source it must not write back.
	aID := insertArtworkTestMapping(t, db, 100, "anilist")
	xID := insertArtworkTestMapping(t, db, 200, "")

	mappings, err := db.GetAllMappings(ctx)
	require.NoError(t, err)
	require.Len(t, mappings, 2)

	refreshDone := make(chan artwork.RefreshAllSummary, 1)
	go func() { refreshDone <- mgr.RefreshAll(ctx, mappings, "anilist", nil) }()
	<-anilist.entered // refresh-all parked mid-run, holding mapping A's fetch

	// User per-show refreshes X with an explicit provider while the
	// refresh-all is in flight — the newer intent that must win.
	cfg := config.DefaultConfig()
	cfg.SonarrURL = ""
	rec := httptest.NewRecorder()
	handleRefreshArtwork(cfg, db, mgr, nil)(rec, requestWithChiID(http.MethodPost, fmt.Sprintf("/api/mappings/%d/artwork/refresh?source=kitsu", xID), fmt.Sprintf("%d", xID), ""))
	require.Equal(t, http.StatusOK, rec.Code)

	close(anilist.release)
	select {
	case <-refreshDone:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for refresh-all")
	}

	// X must keep the newer per-show refresh's kitsu artwork; the older
	// refresh-all write-back must have been skipped.
	assert.Equal(t, "kitsu-jpeg", readArtworkPoster(t, artDir, xID), "refresh-all must not clobber the newer per-show refresh's artwork")
	assert.Equal(t, "anilist-jpeg", readArtworkPoster(t, artDir, aID), "the parked mapping's own refresh should still complete")
}

// TestRefreshAllDoesNotClobberNewerSourceSwitch is the failing-first
// reproduction for the source-switch variant: the user switches a show's
// artwork source while a refresh-all is in flight. The switch records newer
// intent, so the refresh-all must not download the stale default source's
// artwork over the mapping after the switch.
func TestRefreshAllDoesNotClobberNewerSourceSwitch(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir, anilist := newOrderingRaceManager(t, db)
	ctx := context.Background()

	insertArtworkTestMapping(t, db, 100, "anilist")
	xID := insertArtworkTestMapping(t, db, 200, "")

	mappings, err := db.GetAllMappings(ctx)
	require.NoError(t, err)
	require.Len(t, mappings, 2)

	refreshDone := make(chan artwork.RefreshAllSummary, 1)
	go func() { refreshDone <- mgr.RefreshAll(ctx, mappings, "anilist", nil) }()
	<-anilist.entered // refresh-all parked mid-run on mapping A's fetch

	// User switches X's source to kitsu while the refresh-all is in flight.
	// A non-sonarr switch performs no fetch of its own, so X's artwork must
	// simply stay absent — the stale refresh-all must not populate it.
	rec := httptest.NewRecorder()
	handleSetArtworkSource(db, mgr)(rec, requestWithChiID(http.MethodPut, fmt.Sprintf("/api/mappings/%d/artwork/source", xID), fmt.Sprintf("%d", xID), `{"source":"kitsu"}`))
	require.Equal(t, http.StatusOK, rec.Code)

	close(anilist.release)
	select {
	case <-refreshDone:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for refresh-all")
	}

	// The stale refresh-all must not have downloaded the old default source's
	// artwork for X, nor populated its DB paths.
	assertNoArtworkDir(t, artDir, xID)
	current, err := db.GetMappingByID(ctx, xID)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "kitsu", current.ArtworkSource)
	assert.Equal(t, "", current.ArtworkPosterPath, "stale refresh-all must not populate X's poster path")
}

// TestRefreshAllSupersededDownloadLeavesNoTrace exercises the late guard:
// a source switch lands while the refresh-all's own fetch for X is parked in
// the provider (before any download), so the guard fires after the download
// completes. The stale fetch must leave neither files on disk nor DB paths —
// the unchanged DB path must not render a stale-source file over the switch.
func TestRefreshAllSupersededDownloadLeavesNoTrace(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir, anilist := newOrderingRaceManager(t, db)
	ctx := context.Background()

	xID := insertArtworkTestMapping(t, db, 200, "")

	mappings, err := db.GetAllMappings(ctx)
	require.NoError(t, err)
	require.Len(t, mappings, 1)

	refreshDone := make(chan artwork.RefreshAllSummary, 1)
	go func() { refreshDone <- mgr.RefreshAll(ctx, mappings, "anilist", nil) }()
	<-anilist.entered // refresh-all parked inside X's own fetch, pre-download

	// User switches X's source to kitsu while the fetch is in flight. No
	// fetch follows a non-sonarr switch, so only the guard can stop the
	// stale anilist download from landing.
	rec := httptest.NewRecorder()
	handleSetArtworkSource(db, mgr)(rec, requestWithChiID(http.MethodPut, fmt.Sprintf("/api/mappings/%d/artwork/source", xID), fmt.Sprintf("%d", xID), `{"source":"kitsu"}`))
	require.Equal(t, http.StatusOK, rec.Code)

	close(anilist.release)
	select {
	case <-refreshDone:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for refresh-all")
	}

	// The superseded fetch must have removed its downloads and skipped the
	// DB write-back: no files, no paths.
	assertNoArtworkDir(t, artDir, xID)
	current, err := db.GetMappingByID(ctx, xID)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "kitsu", current.ArtworkSource)
	assert.Equal(t, "", current.ArtworkPosterPath, "superseded download must not populate X's poster path")
}

// TestRefreshAllProcessesUnmodifiedMappings is the control: with no user
// intent landing mid-run, refresh-all processes every mapping normally — the
// intent guard must not over-skip.
func TestRefreshAllProcessesUnmodifiedMappings(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir, anilist := newOrderingRaceManager(t, db)
	ctx := context.Background()

	aID := insertArtworkTestMapping(t, db, 100, "anilist")
	xID := insertArtworkTestMapping(t, db, 200, "")

	mappings, err := db.GetAllMappings(ctx)
	require.NoError(t, err)
	require.Len(t, mappings, 2)

	refreshDone := make(chan artwork.RefreshAllSummary, 1)
	go func() { refreshDone <- mgr.RefreshAll(ctx, mappings, "anilist", nil) }()
	<-anilist.entered
	close(anilist.release)
	summary := <-refreshDone

	assert.Equal(t, 2, summary.Succeeded, "unmodified mappings must all succeed")
	assert.Equal(t, 0, summary.NotFound)
	for _, id := range []int64{aID, xID} {
		assert.Equal(t, "anilist-jpeg", readArtworkPoster(t, artDir, id))
	}
}
