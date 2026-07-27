package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"

	"github.com/go-chi/chi/v5"
)

func TestHandleUpdateSettingsRejectsInvalidFillerAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"default_filler_action":"delete"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid filler action, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateSettingsRejectsInvalidMixedCanonAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"default_mixed_canon_action":"banana"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid mixed canon action, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateSettingsAcceptsValidFillerAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"default_filler_action":"unmonitor"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid filler action, got %d: %s", rec.Code, rec.Body.String())
	}
	if cfg.DefaultFillerAction != config.ActionUnmonitor {
		t.Fatalf("expected cfg.DefaultFillerAction to be 'unmonitor', got %q", cfg.DefaultFillerAction)
	}
}

func TestHandleUpdateSettingsPersistsNtfySettings(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"ntfy_enabled":true,"ntfy_server_url":"https://ntfy.example.com","ntfy_topic":"my-topic","ntfy_token":"tok123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify values were applied to cfg
	if !cfg.NtfyEnabled || cfg.NtfyServerURL != "https://ntfy.example.com" || cfg.NtfyTopic != "my-topic" || cfg.NtfyToken != "tok123" {
		t.Fatalf("cfg not updated correctly: %+v", cfg)
	}

	// Verify values were persisted to DB
	val, err := db.GetSetting(req.Context(), "ntfy_enabled")
	if err != nil || val != "true" {
		t.Fatalf("expected ntfy_enabled=true in DB, got %q err=%v", val, err)
	}
	val, err = db.GetSetting(req.Context(), "ntfy_topic")
	if err != nil || val != "my-topic" {
		t.Fatalf("expected ntfy_topic=my-topic in DB, got %q err=%v", val, err)
	}
}

func TestHandleUpdateSettingsRejectsNtfyEnabledWithoutConfig(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"ntfy_enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for enabling ntfy without server URL/topic, got %d: %s", rec.Code, rec.Body.String())
	}
	if cfg.NtfyEnabled {
		t.Fatal("expected NtfyEnabled to remain false after rejected update")
	}
}

func TestHandleGetSettingsReturnsNtfyFields(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	cfg.NtfyEnabled = true
	cfg.NtfyTopic = "test-topic"
	cfg.NtfyToken = "secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handleGetSettings(cfg, db)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if result["ntfy_enabled"] != true {
		t.Fatalf("expected ntfy_enabled=true, got %v", result["ntfy_enabled"])
	}
	if result["ntfy_topic"] != "test-topic" {
		t.Fatalf("expected ntfy_topic=test-topic, got %v", result["ntfy_topic"])
	}
	if result["ntfy_token_set"] != true {
		t.Fatalf("expected ntfy_token_set=true, got %v", result["ntfy_token_set"])
	}
}

func TestHandleCreateMappingRejectsInvalidFillerAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"sonarr_series_id":1,"afl_slug":"naruto","filler_action":"delete"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleCreateMapping(cfg, db, nil)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid filler action, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetMappingArtworkReturnsCachedArtwork(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	mapping := models.ShowMapping{
		SonarrSeriesID:   1,
		SonarrTitle:      "Naruto",
		AFLSlug:          "naruto",
		AFLTitle:         "Naruto",
		FillerAction:     models.ActionUnmonitor,
		MixedAction:      models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor,
		Enabled:          true,
	}
	if err := db.UpsertMapping(ctx, &mapping); err != nil {
		t.Fatalf("creating mapping: %v", err)
	}
	// Fetch back to get the auto-incremented ID
	stored, err := db.GetMappingBySonarrID(ctx, mapping.SonarrSeriesID)
	if err != nil || stored == nil {
		t.Fatalf("loading saved mapping: %v", err)
	}
	mapping = *stored
	if err := db.UpdateAnilistArtwork(ctx, mapping.ID, "https://example.com/poster.jpg", "https://example.com/banner.jpg"); err != nil {
		t.Fatalf("setting artwork: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.EnableAniListArtwork = true

	engine := syncengine.New(cfg, db)

	req := httptest.NewRequest(http.MethodGet, "/api/mappings/1/artwork", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(mapping.ID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handleGetMappingArtwork(cfg, db, engine)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if result["poster_image"] != "https://example.com/poster.jpg" {
		t.Fatalf("expected cached poster URL, got %v", result["poster_image"])
	}
}
