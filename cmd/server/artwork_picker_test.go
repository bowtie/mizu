package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

// stubCandidatesProvider implements both Provider and CandidatesProvider,
// serving a fixed candidate set without any network access.
type stubCandidatesProvider struct {
	opts *artwork.ArtworkOptions
}

func (p *stubCandidatesProvider) Name() string    { return "fanart" }
func (p *stubCandidatesProvider) Available() bool { return true }
func (p *stubCandidatesProvider) Fetch(context.Context, models.SonarrSeries) (*artwork.Artwork, error) {
	return nil, nil
}
func (p *stubCandidatesProvider) Candidates(context.Context, models.SonarrSeries) (*artwork.ArtworkOptions, error) {
	return p.opts, nil
}

// newPickerTestManager builds a manager registered with a stub "fanart"
// provider, mirroring the picker's only dependency: a concrete source name.
func newPickerTestManager(t *testing.T, db *database.DB, opts *artwork.ArtworkOptions) (mgr *artwork.Manager, artDir string) {
	t.Helper()
	artDir = filepath.Join(t.TempDir(), "artwork")
	mgr = artwork.NewManager(artDir, db)
	mgr.Register(&stubCandidatesProvider{opts: opts})
	return mgr, artDir
}

// TestListArtworkCandidatesReturnsOptions is the happy-path control for the
// picker: the handler returns the provider's candidate posters/banners/logos.
func TestListArtworkCandidatesReturnsOptions(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	opts := &artwork.ArtworkOptions{
		Posters: []artwork.ArtworkOption{
			{URL: "https://example.com/p1.jpg", ThumbURL: "https://example.com/p1_t.jpg", Lang: "en", Likes: 3},
			{URL: "https://example.com/p2.jpg", Lang: "ja", Likes: 1},
		},
		Banners: []artwork.ArtworkOption{{URL: "https://example.com/b1.jpg", Width: 1920}},
		Logos:   []artwork.ArtworkOption{{URL: "https://example.com/logo.svg"}},
	}
	mgr, _ := newPickerTestManager(t, db, opts)
	id := insertArtworkTestMapping(t, db, 7, "")

	rec := httptest.NewRecorder()
	handleListArtworkCandidates(config.DefaultConfig(), db, mgr, nil)(
		rec,
		requestWithChiID(http.MethodGet, "/api/mappings/1/artwork/candidates?source=fanart", fmt.Sprintf("%d", id), ""),
	)

	require.Equal(t, http.StatusOK, rec.Code)
	var got artwork.ArtworkOptions
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got.Posters, 2)
	assert.Equal(t, "https://example.com/p1_t.jpg", got.Posters[0].ThumbURL)
	assert.Len(t, got.Banners, 1)
	assert.Len(t, got.Logos, 1)
}

// TestListArtworkCandidatesRequiresSource is a control: the endpoint rejects
// a request without a `source` query parameter instead of fetching nothing.
func TestListArtworkCandidatesRequiresSource(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	mgr, _ := newPickerTestManager(t, db, nil)
	id := insertArtworkTestMapping(t, db, 7, "")

	rec := httptest.NewRecorder()
	handleListArtworkCandidates(config.DefaultConfig(), db, mgr, nil)(
		rec,
		requestWithChiID(http.MethodGet, "/api/mappings/1/artwork/candidates", fmt.Sprintf("%d", id), ""),
	)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSelectArtworkDownloadsAndPersists is the happy-path control: picking an
// image downloads it to the mapping's artwork dir, writes the local DB path,
// and sets the per-show source override so the pick sticks.
func TestSelectArtworkDownloadsAndPersists(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	imgSrv := artworkImageServer(t, "picked-jpeg")

	mgr, artDir := newPickerTestManager(t, db, nil)
	id := insertArtworkTestMapping(t, db, 7, "")

	body := fmt.Sprintf(`{"source":"fanart","poster_url":"%s/poster.jpg","banner_url":"","logo_url":""}`, imgSrv.URL)
	rec := httptest.NewRecorder()
	handleSelectArtwork(db, mgr)(rec, requestWithChiID(http.MethodPut, "/api/mappings/1/artwork/select", fmt.Sprintf("%d", id), body))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "picked-jpeg", readArtworkPoster(t, artDir, id))

	current, err := db.GetMappingByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "fanart", current.ArtworkSource, "pick must set the per-show source override")
	assert.NotEmpty(t, current.ArtworkPosterPath, "pick must persist the local poster path")
	assert.Equal(t, imgSrv.URL+"/poster.jpg", current.ArtworkPosterURL, "pick must persist the source URL so the picker can ring the active tile")
}

// TestSelectArtworkPreservesUnpickedSlots is the partial-pick control:
// picking only one slot (poster) must not wipe the other slots' existing
// files or DB paths.
func TestSelectArtworkPreservesUnpickedSlots(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	bannerSrv := artworkImageServer(t, "banner-jpeg")
	posterSrv := artworkImageServer(t, "poster-jpeg")

	mgr, artDir := newPickerTestManager(t, db, nil)
	id := insertArtworkTestMapping(t, db, 7, "")

	// Seed a banner-only pick first.
	_, err = mgr.SelectArtwork(context.Background(), id, "fanart", "", bannerSrv.URL+"/banner.jpg", "")
	require.NoError(t, err)

	// Now pick only a poster.
	_, err = mgr.SelectArtwork(context.Background(), id, "fanart", posterSrv.URL+"/poster.jpg", "", "")
	require.NoError(t, err)

	bannerData, err := os.ReadFile(filepath.Join(artDir, fmt.Sprintf("%d", id), "banner.jpg"))
	require.NoError(t, err, "unpicked banner file must survive a poster-only pick")
	assert.Equal(t, "banner-jpeg", string(bannerData))
	assert.Equal(t, "poster-jpeg", readArtworkPoster(t, artDir, id))

	current, err := db.GetMappingByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.NotEmpty(t, current.ArtworkBannerPath, "unpicked banner DB path must survive")
	assert.NotEmpty(t, current.ArtworkPosterPath)
	assert.Equal(t, bannerSrv.URL+"/banner.jpg", current.ArtworkBannerURL, "unpicked banner source URL must survive")
	assert.Equal(t, posterSrv.URL+"/poster.jpg", current.ArtworkPosterURL)
}

// TestSelectArtworkSupersededByNewerIntent is the failing-first reproduction
// of the picker's ordering race: a pick is downloading when a newer intent
// (e.g. a per-show refresh) lands. The older pick must leave no files, no DB
// paths, and no override change — the newer operation wins.
func TestSelectArtworkSupersededByNewerIntent(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	// A parking image server blocks the download so a newer intent can land
	// while SelectArtwork is mid-flight.
	entered := make(chan struct{})
	release := make(chan struct{})
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		close(entered)
		<-release
		_, _ = w.Write([]byte("slow-jpeg"))
	}))
	t.Cleanup(imgSrv.Close)

	mgr, artDir := newPickerTestManager(t, db, nil)
	id := insertArtworkTestMapping(t, db, 7, "")

	body := fmt.Sprintf(`{"source":"fanart","poster_url":"%s/poster.jpg","banner_url":"","logo_url":""}`, imgSrv.URL)
	done := make(chan struct{})
	go func() {
		defer close(done)
		rec := httptest.NewRecorder()
		handleSelectArtwork(db, mgr)(rec, requestWithChiID(http.MethodPut, "/api/mappings/1/artwork/select", fmt.Sprintf("%d", id), body))
	}()

	// Wait for the download to actually start, then supersede it.
	waitDone(t, entered, "select download to start")
	mgr.BumpIntent(id)
	close(release)
	waitDone(t, done, "select handler to finish")

	// The superseded pick must leave no trace: no files, no DB paths, and the
	// override must remain whatever it was (empty here) rather than "fanart".
	assertNoArtworkDir(t, artDir, id)
	current, err := db.GetMappingByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "", current.ArtworkPosterPath, "superseded pick must not write DB paths")
	assert.Equal(t, "", current.ArtworkPosterURL, "superseded pick must not write DB source URLs")
	assert.Equal(t, "", current.ArtworkSource, "superseded pick must not overwrite the source override")
}
