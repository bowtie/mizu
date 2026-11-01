package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

// newSonarrClearManager builds a manager with two static providers (anilist,
// kitsu) serving distinguishable posters, so a test can seed stale local
// artwork and later prove a sonarr-resolved refresh cleared it (or that an
// explicit provider override was left untouched).
func newSonarrClearManager(t *testing.T, db *database.DB) (mgr *artwork.Manager, artDir string) {
	t.Helper()
	anilistSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("anilist-jpeg"))
	}))
	t.Cleanup(anilistSrv.Close)
	kitsuSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("kitsu-jpeg"))
	}))
	t.Cleanup(kitsuSrv.Close)

	artDir = filepath.Join(t.TempDir(), "artwork")
	mgr = artwork.NewManager(artDir, db)
	mgr.Register(&staticArtworkProvider{posterURL: anilistSrv.URL + "/poster.jpg"})
	mgr.Register(&namedStaticArtworkProvider{name: "kitsu", posterURL: kitsuSrv.URL + "/poster.jpg"})
	return mgr, artDir
}

// TestRefreshResolvedSonarrClearsStaleArtwork is the failing-first
// reproduction: a mapping downloaded artwork from a provider, and a refresh
// that resolves to the "sonarr" global default skips the fetch. The stale
// local files and DB paths must be cleared so the sonarr source actually
// takes effect instead of the old downloads rendering over the Sonarr URLs.
func TestRefreshResolvedSonarrClearsStaleArtwork(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir := newSonarrClearManager(t, db)
	ctx := context.Background()
	id := insertArtworkTestMapping(t, db, 100, "")

	// Seed stale local artwork from a provider fetch.
	res, err := mgr.FetchAndStore(ctx, id, models.SonarrSeries{ID: 100, Title: "Test Show"}, "anilist")
	require.NoError(t, err)
	require.True(t, res.Found)
	require.Equal(t, "anilist-jpeg", readArtworkPoster(t, artDir, id))

	// Refresh with no source query param: the global default "sonarr"
	// resolves, the fetch is skipped, and the stale artwork must be cleared.
	cfg := config.DefaultConfig()
	cfg.SonarrURL = ""
	cfg.ArtworkDefaultSource = "sonarr"
	rec := httptest.NewRecorder()
	handleRefreshArtwork(cfg, db, mgr, nil)(rec, requestWithChiID(http.MethodPost, fmt.Sprintf("/api/mappings/%d/artwork/refresh", id), fmt.Sprintf("%d", id), ""))
	require.Equal(t, http.StatusOK, rec.Code)

	assertNoArtworkDir(t, artDir, id)
	current, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "", current.ArtworkPosterPath, "sonarr-resolved refresh must clear the poster path")
	assert.Equal(t, "", current.ArtworkBannerPath, "sonarr-resolved refresh must clear the banner path")
	assert.Equal(t, "", current.ArtworkLogoPath, "sonarr-resolved refresh must clear the logo path")
}

// TestRefreshAllSonarrDefaultClearsStaleButKeepsOverrides is the failing-first
// reproduction for refresh-all with a sonarr global default: mappings without
// an override resolve to sonarr and must have their stale artwork cleared,
// while a mapping with an explicit provider override is left untouched.
func TestRefreshAllSonarrDefaultClearsStaleButKeepsOverrides(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	mgr, artDir := newSonarrClearManager(t, db)
	ctx := context.Background()

	// A: no override -> resolves to the sonarr global default -> must clear.
	aID := insertArtworkTestMapping(t, db, 100, "")
	// B: explicit kitsu override -> must keep its provider artwork. The
	// upsertMapping query doesn't persist artwork_source (it's managed via
	// UpdateArtworkSource), so set the override through that dedicated path.
	bID := insertArtworkTestMapping(t, db, 200, "")
	require.NoError(t, db.UpdateArtworkSource(ctx, bID, "kitsu"))

	for _, m := range []struct {
		id     int64
		source string
	}{
		{aID, "anilist"},
		{bID, "kitsu"},
	} {
		res, err := mgr.FetchAndStore(ctx, m.id, models.SonarrSeries{ID: int(m.id), Title: "Test Show"}, m.source)
		require.NoError(t, err)
		require.True(t, res.Found, "seeding artwork for mapping %d", m.id)
	}

	mappings, err := db.GetAllMappings(ctx)
	require.NoError(t, err)
	require.Len(t, mappings, 2)

	summary := mgr.RefreshAll(ctx, mappings, "sonarr", nil)
	assert.Equal(t, 1, summary.Succeeded, "only the explicit-override mapping fetches")
	assert.Equal(t, 1, summary.NotFound, "the sonarr-resolved mapping is a no-op")

	// A: stale provider artwork cleared.
	assertNoArtworkDir(t, artDir, aID)
	currentA, err := db.GetMappingByID(ctx, aID)
	require.NoError(t, err)
	require.NotNil(t, currentA)
	assert.Equal(t, "", currentA.ArtworkPosterPath, "sonarr-resolved mapping must have its poster path cleared")

	// B: explicit override untouched — artwork still present and pathed.
	assert.Equal(t, "kitsu-jpeg", readArtworkPoster(t, artDir, bID))
	currentB, err := db.GetMappingByID(ctx, bID)
	require.NoError(t, err)
	require.NotNil(t, currentB)
	assert.Equal(t, "kitsu", currentB.ArtworkSource)
	assert.NotEmpty(t, currentB.ArtworkPosterPath, "explicit-override mapping keeps its poster path")
}
