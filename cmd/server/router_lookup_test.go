package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"
)

func TestHandleLookupSonarrSeriesKeepsStandardLookupResults(t *testing.T) {
	sonarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series/lookup" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("term"); got != "Ranma 1/2" {
			t.Fatalf("unexpected lookup term %q", got)
		}
		_ = json.NewEncoder(w).Encode([]models.SonarrSeries{
			{Title: "Ranma 1/2", SeriesType: "standard", TVDBID: 1},
			{Title: "Bleach", SeriesType: "anime", TVDBID: 2},
		})
	}))
	defer sonarr.Close()

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		SonarrURL:    sonarr.URL,
		SonarrAPIKey: "test-key",
	}
	engine := syncengine.New(cfg, db)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/lookup?q=Ranma%201%2F2", nil)
	rec := httptest.NewRecorder()

	handleLookupSonarrSeries(engine)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}

	var got []models.SonarrSeries
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected both standard and anime lookup results, got %+v", got)
	}
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

	if got != "/data/anime" {
		t.Fatalf("preferredSonarrRootFolder() = %q, want /data/anime", got)
	}
}

func TestPreferredSonarrRootFolderFallsBackToFirstRootFolder(t *testing.T) {
	got := preferredSonarrRootFolder(nil, []models.SonarrRootFolder{
		{Path: "/data/default"},
		{Path: "/data/secondary"},
	})

	if got != "/data/default" {
		t.Fatalf("preferredSonarrRootFolder() = %q, want /data/default", got)
	}
}
