package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"
)

func TestHandleLookupSonarrSeriesKeepsStandardLookupResults(t *testing.T) {
	sonarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/series/lookup", r.URL.Path)
		assert.Equal(t, "Ranma 1/2", r.URL.Query().Get("term"))
		_ = json.NewEncoder(w).Encode([]models.SonarrSeries{
			{Title: "Ranma 1/2", SeriesType: "standard", TVDBID: 1},
			{Title: "Bleach", SeriesType: "anime", TVDBID: 2},
		})
	}))
	defer sonarr.Close()

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := &config.Config{
		SonarrURL:    sonarr.URL,
		SonarrAPIKey: "test-key",
	}
	engine := syncengine.New(cfg, db)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/lookup?q=Ranma%201%2F2", nil)
	rec := httptest.NewRecorder()

	handleLookupSonarrSeries(engine)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []models.SonarrSeries
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestPreferredSonarrRootFolderPrefersExistingAnimeLibraryRoot(t *testing.T) {
	got := preferredSonarrRootFolder(
		[]models.SonarrSeries{
			{RootFolderPath: "/data/anime"},
			{RootFolderPath: "/data/anime"},
			{RootFolderPath: "/data/other"},
		},
		[]models.SonarrRootFolder{
			{Path: "/data/default"},
		},
	)

	assert.Equal(t, "/data/anime", got)
}

func TestPreferredSonarrRootFolderFallsBackToFirstRootFolder(t *testing.T) {
	got := preferredSonarrRootFolder(nil, []models.SonarrRootFolder{
		{Path: "/data/default"},
		{Path: "/data/secondary"},
	})

	assert.Equal(t, "/data/default", got)
}
