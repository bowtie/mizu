package artwork

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	"mizu/internal/models"
)

// ── Test doubles ──

// staticProvider returns artwork immediately under a fixed name.
type staticProvider struct {
	name      string
	posterURL string
}

func (p *staticProvider) Name() string    { return p.name }
func (p *staticProvider) Available() bool { return true }
func (p *staticProvider) Fetch(context.Context, models.SonarrSeries) (*Artwork, error) {
	return &Artwork{PosterURL: p.posterURL, Source: p.name}, nil
}

// blockingProvider parks its fetch until release, so a test can hold the
// per-mapping lock mid-download and then inject a newer intent (or a delete).
type blockingProvider struct {
	enteredOnce sync.Once
	entered     chan struct{}
	release     chan struct{}
	posterURL   string
}

func (p *blockingProvider) Name() string    { return "anilist" }
func (p *blockingProvider) Available() bool { return true }
func (p *blockingProvider) Fetch(ctx context.Context, _ models.SonarrSeries) (*Artwork, error) {
	p.enteredOnce.Do(func() { close(p.entered) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.release:
	}
	return &Artwork{PosterURL: p.posterURL, Source: "anilist"}, nil
}

// testImageServer serves a distinguishable JPEG body so tests can tell which
// provider's download won.
func testImageServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ── Setup helpers ──

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newStaticManager(t *testing.T, db *database.DB) (*Manager, string) {
	t.Helper()
	imgSrv := testImageServer(t, "fake-jpeg")
	artDir := filepath.Join(t.TempDir(), "artwork")
	mgr := NewManager(artDir, db)
	mgr.Register(&staticProvider{name: "anilist", posterURL: imgSrv.URL + "/poster.jpg"})
	return mgr, artDir
}

func newBlockingManager(t *testing.T, db *database.DB) (*Manager, string, *blockingProvider) {
	t.Helper()
	imgSrv := testImageServer(t, "fake-jpeg")
	artDir := filepath.Join(t.TempDir(), "artwork")
	mgr := NewManager(artDir, db)
	p := &blockingProvider{
		entered:   make(chan struct{}),
		release:   make(chan struct{}),
		posterURL: imgSrv.URL + "/poster.jpg",
	}
	mgr.Register(p)
	return mgr, artDir, p
}

// insertMapping upserts a mapping and returns its ID. The override is NOT
// persisted by UpsertMapping (artwork_source is managed via UpdateArtworkSource),
// so tests that need an override call UpdateArtworkSource separately.
func insertMapping(t *testing.T, db *database.DB, seriesID int) int64 {
	t.Helper()
	m := &models.ShowMapping{
		SonarrSeriesID: seriesID,
		SonarrTitle:    "Test Show",
		AFLSlug:        "test-show",
		Enabled:        true,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	require.NotZero(t, m.ID, "UpsertMapping should populate the mapping ID")
	return m.ID
}

func testSeries(seriesID int) models.SonarrSeries {
	return models.SonarrSeries{ID: seriesID, Title: "Test Show"}
}

func readPoster(t *testing.T, artDir string, mappingID int64) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(artDir, fmt.Sprintf("%d", mappingID), "poster.jpg"))
	require.NoError(t, err, "mapping %d should have a downloaded poster", mappingID)
	return string(data)
}

func assertNoArtworkDir(t *testing.T, artDir string, mappingID int64) {
	t.Helper()
	_, err := os.Stat(filepath.Join(artDir, fmt.Sprintf("%d", mappingID)))
	assert.True(t, os.IsNotExist(err), "artwork dir %d should not exist, got: %v", mappingID, err)
}

func assertNoDBPaths(t *testing.T, db *database.DB, mappingID int64) {
	t.Helper()
	m, err := db.GetMappingByID(context.Background(), mappingID)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "", m.ArtworkPosterPath, "poster path must be cleared")
	assert.Equal(t, "", m.ArtworkBannerPath, "banner path must be cleared")
	assert.Equal(t, "", m.ArtworkLogoPath, "logo path must be cleared")
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// ── Per-mapping lock ──

// TestLockMappingIsPerMapping verifies the lock map's core contract: fetches
// for the same mapping share one mutex, while different mappings serialize
// independently.
func TestLockMappingIsPerMapping(t *testing.T) {
	mgr := NewManager(t.TempDir(), nil)

	same := mgr.lockMapping(1)
	again := mgr.lockMapping(1)
	other := mgr.lockMapping(2)

	if same != again {
		t.Fatal("the same mapping must reuse a single mutex")
	}
	if same == other {
		t.Fatal("different mappings must have distinct mutexes")
	}
}

// TestDeleteArtworkSerializesWithInFlightFetch is the package-level
// reproduction of the delete-resurrection guard: DeleteArtwork takes the same
// per-mapping lock a fetch holds, so a fetch in flight must finish before the
// delete runs — and the delete then removes the fetch's output.
func TestDeleteArtworkSerializesWithInFlightFetch(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir, p := newBlockingManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	fetchDone := make(chan error, 1)
	go func() {
		_, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
		fetchDone <- err
	}()
	waitFor(t, p.entered, "fetch to hold the per-mapping lock")

	// DeleteArtwork must block behind the fetch, not run and let the fetch
	// resurrect files afterwards.
	deleteDone := make(chan struct{})
	go func() {
		mgr.DeleteArtwork(id)
		close(deleteDone)
	}()
	select {
	case <-deleteDone:
		t.Fatal("DeleteArtwork must block while a fetch holds the per-mapping lock")
	case <-time.After(200 * time.Millisecond):
		// Good: the delete is serialized behind the in-flight fetch.
	}

	close(p.release)
	require.NoError(t, <-fetchDone)
	waitFor(t, deleteDone, "delete to run after the fetch")

	assertNoArtworkDir(t, artDir, id)
}

// ── Happy path ──

func TestFetchAndStoreDownloadsAndPersists(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newStaticManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	res, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Found)
	assert.Equal(t, "anilist", res.Source)

	assert.Equal(t, "fake-jpeg", readPoster(t, artDir, id))

	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, filepath.Join(fmt.Sprintf("%d", id), "poster.jpg"), m.ArtworkPosterPath)
	assert.True(t, strings.HasSuffix(m.ArtworkPosterURL, "/poster.jpg"), "fetch must persist the source URL so the picker can ring the active tile, got %q", m.ArtworkPosterURL)
}

// ── Intent supersede ──

// TestFetchAndStoreSkipsWhenSupersededEarly models a refresh-all write-back
// whose intent snapshot predates a newer per-show refresh: the fetch must
// skip entirely, leaving no files or DB paths.
func TestFetchAndStoreSkipsWhenSupersededEarly(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newStaticManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	staleIntent := mgr.captureIntent(id)
	mgr.BumpIntent(id) // a newer per-show refresh lands

	res, err := mgr.fetchAndStore(ctx, id, testSeries(100), "anilist", staleIntent)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.Found)

	assertNoArtworkDir(t, artDir, id)
	assertNoDBPaths(t, db, id)
}

// TestFetchAndStoreDiscardsDownloadWhenSupersededMidFlight is the late-guard
// case: the fetch is already downloading when a newer intent lands, so the
// stale fetch must discard its download and skip the DB write-back.
func TestFetchAndStoreDiscardsDownloadWhenSupersededMidFlight(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir, p := newBlockingManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	type outcome struct {
		res *FetchResult
		err error
	}
	staleIntent := mgr.captureIntent(id)
	done := make(chan outcome, 1)
	go func() {
		res, err := mgr.fetchAndStore(ctx, id, testSeries(100), "anilist", staleIntent)
		done <- outcome{res: res, err: err}
	}()
	waitFor(t, p.entered, "fetch to park in the provider")

	mgr.BumpIntent(id) // newer intent lands mid-download
	close(p.release)

	out := <-done
	require.NoError(t, out.err)
	require.NotNil(t, out.res)
	assert.False(t, out.res.Found)

	assertNoArtworkDir(t, artDir, id)
	assertNoDBPaths(t, db, id)
}

// ── Sonarr clear ──

// TestClearArtworkForSonarr verifies the public sonarr-switch cleanup: files
// are removed and the DB file paths are cleared.
func TestClearArtworkForSonarr(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newStaticManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	res, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	require.True(t, res.Found)
	assert.Equal(t, "fake-jpeg", readPoster(t, artDir, id))

	require.NoError(t, mgr.ClearArtworkForSonarr(ctx, id))
	assertNoArtworkDir(t, artDir, id)
	assertNoDBPaths(t, db, id)
}

// TestFetchAndStoreSonarrSourceClearsStaleArtwork covers the path where a
// fetch resolves to "sonarr" (the global default): the download is skipped but
// stale local files and DB paths must be cleared so Sonarr URLs take effect.
func TestFetchAndStoreSonarrSourceClearsStaleArtwork(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newStaticManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	res, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	require.True(t, res.Found)

	res, err = mgr.FetchAndStore(ctx, id, testSeries(100), "sonarr")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.Found)

	assertNoArtworkDir(t, artDir, id)
	assertNoDBPaths(t, db, id)
}

// TestFetchAndStoreSonarrOverrideClearsRegardlessOfIntent covers the override
// path: when the mapping's override is "sonarr", stale local artwork is stale
// by definition, so the clear runs even if a newer intent has landed (the
// sonarr check is intentionally not gated on intent).
func TestFetchAndStoreSonarrOverrideClearsRegardlessOfIntent(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newStaticManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	res, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	require.True(t, res.Found)

	require.NoError(t, db.UpdateArtworkSource(ctx, id, "sonarr"))

	// A stale intent must not stop the sonarr clear.
	staleIntent := mgr.captureIntent(id)
	mgr.BumpIntent(id)

	res, err = mgr.fetchAndStore(ctx, id, testSeries(100), "anilist", staleIntent)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.Found)

	assertNoArtworkDir(t, artDir, id)
	assertNoDBPaths(t, db, id)
}
