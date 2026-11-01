package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/artwork"
	"mizu/internal/database"
	"mizu/internal/models"
)

// blockingArtworkProvider models an artwork fetch that is in-flight: it
// signals when it has been entered (the per-mapping lock is held) and
// blocks until released, so tests can interleave a delete or source
// switch with a fetch that has not yet written anything.
type blockingArtworkProvider struct {
	entered   chan struct{}
	release   chan struct{}
	posterURL string
}

func (p *blockingArtworkProvider) Name() string    { return "anilist" }
func (p *blockingArtworkProvider) Available() bool { return true }
func (p *blockingArtworkProvider) Fetch(ctx context.Context, _ models.SonarrSeries) (*artwork.Artwork, error) {
	close(p.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.release:
	}
	return &artwork.Artwork{PosterURL: p.posterURL, Source: "anilist"}, nil
}

// staticArtworkProvider returns artwork immediately, for fetches that
// should not block (e.g. proving a fetch that starts after a delete
// writes nothing).
type staticArtworkProvider struct {
	posterURL string
}

func (p *staticArtworkProvider) Name() string    { return "anilist" }
func (p *staticArtworkProvider) Available() bool { return true }
func (p *staticArtworkProvider) Fetch(context.Context, models.SonarrSeries) (*artwork.Artwork, error) {
	return &artwork.Artwork{PosterURL: p.posterURL, Source: "anilist"}, nil
}

// newArtworkTestManager builds a manager (with its artDir), a fake image
// server, and a blocking provider, mirroring setupRouter's wiring.
func newArtworkTestManager(t *testing.T, db *database.DB) (mgr *artwork.Manager, artDir string, p *blockingArtworkProvider) {
	t.Helper()
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg"))
	}))
	t.Cleanup(imgSrv.Close)

	artDir = filepath.Join(t.TempDir(), "artwork")
	mgr = artwork.NewManager(artDir, db)
	p = &blockingArtworkProvider{
		entered:   make(chan struct{}),
		release:   make(chan struct{}),
		posterURL: imgSrv.URL + "/poster.jpg",
	}
	mgr.Register(p)
	return mgr, artDir, p
}

func insertArtworkTestMapping(t *testing.T, db *database.DB, sonarrSeriesID int, source string) int64 {
	t.Helper()
	m := &models.ShowMapping{
		SonarrSeriesID: sonarrSeriesID,
		SonarrTitle:    "Test Show",
		AFLSlug:        "test-show",
		Enabled:        true,
		ArtworkSource:  source,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	require.NotZero(t, m.ID, "UpsertMapping should populate the mapping ID")
	return m.ID
}

// requestWithChiID attaches the chi "id" URL parameter the handlers
// read via chi.URLParam, as chi.Router does in production.
func requestWithChiID(method, target, id, body string) *http.Request {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, r)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Keys = []string{"id"}
	rctx.URLParams.Values = []string{id}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func waitDone(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// waitHandlerThenRelease waits until the concurrently-running handler has
// either finished or is provably blocked on the per-mapping lock, then
// releases the in-flight fetch. In the failing (pre-fix) code the handler
// completes while the fetch is still blocked in the provider, so releasing
// after handlerDone makes the resurrection deterministic: the fetch's writes
// land after the handler's cleanup. In the fixed code the handler blocks on
// the per-mapping lock (held by the fetch), so handlerDone only closes after
// the fetch finishes — we release once the handler has had ample time to
// reach that block, and the lock ordering guarantees the handler's cleanup
// then runs after the fetch's writes.
func waitHandlerThenRelease(t *testing.T, handlerDone <-chan struct{}, release chan struct{}) {
	t.Helper()
	select {
	case <-handlerDone:
		// Pre-fix: handler already ran its cleanup; nothing else to wait for.
	case <-time.After(250 * time.Millisecond):
		// Fixed: handler is blocked on the per-mapping lock held by the fetch.
	}
	close(release)
}

func assertNoArtworkDir(t *testing.T, artDir string, mappingID int64) {
	t.Helper()
	_, err := os.Stat(filepath.Join(artDir, fmt.Sprintf("%d", mappingID)))
	assert.True(t, os.IsNotExist(err), "artwork dir %d should not exist, got: %v", mappingID, err)
}

// TestHandleDeleteMappingDoesNotResurrectArtworkMidFlight is the failing-first
// reproduction for the delete resurrection: an artwork fetch is in flight
// (holding the per-mapping lock) when the delete handler runs. The fetch must
// not re-create artwork/{id}/ files after the delete removed them.
func TestHandleDeleteMappingDoesNotResurrectArtworkMidFlight(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir, p := newArtworkTestManager(t, db)
	id := insertArtworkTestMapping(t, db, 100, "")

	ctx := context.Background()
	fetchDone := make(chan struct{})
	go func() {
		defer close(fetchDone)
		_, _ = mgr.FetchAndStore(ctx, id, models.SonarrSeries{ID: 100, Title: "Test Show"}, "anilist")
	}()
	<-p.entered // fetch in-flight, per-mapping lock held

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		rec := httptest.NewRecorder()
		handleDeleteMapping(db, mgr)(rec, requestWithChiID(http.MethodDelete, fmt.Sprintf("/api/mappings/%d", id), fmt.Sprintf("%d", id), ""))
		assert.Equal(t, http.StatusOK, rec.Code)
	}()

	// Wait until the handler is done (pre-fix) or blocked on the per-mapping
	// lock (fixed), then release the fetch so the delete wins deterministically.
	waitHandlerThenRelease(t, handlerDone, p.release)
	waitDone(t, fetchDone, "fetch to finish")
	waitDone(t, handlerDone, "delete handler to finish")

	// The delete must win: no artwork files may remain on disk.
	assertNoArtworkDir(t, artDir, id)
}

// TestHandleSetArtworkSourceSonarrDoesNotUndoMidFlightFetch is the failing-first
// reproduction for the source-switch resurrection: an artwork fetch is in
// flight when the source is switched to "sonarr". The fetch must not
// re-populate local files or DB paths, silently undoing the switch.
func TestHandleSetArtworkSourceSonarrDoesNotUndoMidFlightFetch(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir, p := newArtworkTestManager(t, db)
	id := insertArtworkTestMapping(t, db, 200, "")

	ctx := context.Background()
	fetchDone := make(chan struct{})
	go func() {
		defer close(fetchDone)
		_, _ = mgr.FetchAndStore(ctx, id, models.SonarrSeries{ID: 200, Title: "Test Show"}, "anilist")
	}()
	<-p.entered // fetch in-flight, per-mapping lock held

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		rec := httptest.NewRecorder()
		handleSetArtworkSource(db, mgr)(rec, requestWithChiID(http.MethodPut, fmt.Sprintf("/api/mappings/%d/artwork/source", id), fmt.Sprintf("%d", id), `{"source":"sonarr"}`))
		assert.Equal(t, http.StatusOK, rec.Code)
	}()

	// Wait until the handler is done (pre-fix) or blocked on the per-mapping
	// lock (fixed), then release the fetch so the switch wins deterministically.
	waitHandlerThenRelease(t, handlerDone, p.release)
	waitDone(t, fetchDone, "fetch to finish")
	waitDone(t, handlerDone, "source switch handler to finish")

	// The switch to "sonarr" must win: no local files and no DB paths.
	assertNoArtworkDir(t, artDir, id)

	current, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "", current.ArtworkPosterPath, "poster path must be cleared after switch to sonarr")
	assert.Equal(t, "", current.ArtworkBannerPath, "banner path must be cleared after switch to sonarr")
	assert.Equal(t, "", current.ArtworkLogoPath, "logo path must be cleared after switch to sonarr")
}

// TestFetchAndStoreWritesNothingForDeletedMapping proves the supersede guard:
// a fetch that starts after the mapping was deleted (e.g. a refresh-all
// goroutine reaching a mapping deleted mid-run) must not write any files.
func TestFetchAndStoreWritesNothingForDeletedMapping(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg"))
	}))
	defer imgSrv.Close()

	artDir := filepath.Join(t.TempDir(), "artwork")
	mgr := artwork.NewManager(artDir, db)
	mgr.Register(&staticArtworkProvider{posterURL: imgSrv.URL + "/poster.jpg"})
	id := insertArtworkTestMapping(t, db, 300, "")

	ctx := context.Background()
	require.NoError(t, db.DeleteMapping(ctx, id))
	mgr.DeleteArtwork(id)

	result, err := mgr.FetchAndStore(ctx, id, models.SonarrSeries{ID: 300, Title: "Test Show"}, "anilist")
	require.NoError(t, err)
	assert.False(t, result.Found, "a fetch for a deleted mapping should not be found")
	assertNoArtworkDir(t, artDir, id)
}

// TestFetchAndStoreSkipsMappingSwitchedToSonarr proves the supersede guard
// for the source-switch path: a fetch scheduled before the switch to "sonarr"
// but starting after it (e.g. the on-create goroutine still resolving the
// Sonarr series) must not re-populate local files or DB paths with a stale
// source, undoing the switch.
func TestFetchAndStoreSkipsMappingSwitchedToSonarr(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg"))
	}))
	defer imgSrv.Close()

	artDir := filepath.Join(t.TempDir(), "artwork")
	mgr := artwork.NewManager(artDir, db)
	mgr.Register(&staticArtworkProvider{posterURL: imgSrv.URL + "/poster.jpg"})
	id := insertArtworkTestMapping(t, db, 400, "")

	ctx := context.Background()
	// Switch to "sonarr" first, fully (override + cleared files/paths).
	rec := httptest.NewRecorder()
	handleSetArtworkSource(db, mgr)(rec, requestWithChiID(http.MethodPut, fmt.Sprintf("/api/mappings/%d/artwork/source", id), fmt.Sprintf("%d", id), `{"source":"sonarr"}`))
	require.Equal(t, http.StatusOK, rec.Code)

	// A fetch with the stale pre-switch source must now write nothing.
	result, err := mgr.FetchAndStore(ctx, id, models.SonarrSeries{ID: 400, Title: "Test Show"}, "anilist")
	require.NoError(t, err)
	assert.False(t, result.Found, "a fetch for a mapping switched to sonarr should not be found")
	assertNoArtworkDir(t, artDir, id)

	current, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "sonarr", current.ArtworkSource)
	assert.Equal(t, "", current.ArtworkPosterPath, "stale fetch must not re-populate DB paths")
}
