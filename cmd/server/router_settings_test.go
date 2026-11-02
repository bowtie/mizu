package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

func TestHandleUpdateSettingsRejectsInvalidFillerAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"default_filler_action":"delete"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleUpdateSettingsRejectsInvalidMixedCanonAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"default_mixed_canon_action":"banana"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleUpdateSettingsAcceptsValidFillerAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"default_filler_action":"unmonitor"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, config.ActionUnmonitor, cfg.DefaultFillerAction)
}

func TestHandleUpdateSettingsPersistsNtfySettings(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"ntfy_enabled":true,"ntfy_server_url":"https://ntfy.example.com","ntfy_topic":"my-topic","ntfy_token":"tok123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	assert.True(t, cfg.NtfyEnabled)
	assert.Equal(t, "https://ntfy.example.com", cfg.NtfyServerURL)
	assert.Equal(t, "my-topic", cfg.NtfyTopic)
	assert.Equal(t, "tok123", cfg.NtfyToken)

	val, err := db.GetSetting(req.Context(), "ntfy_enabled")
	require.NoError(t, err)
	assert.Equal(t, "true", val)

	val, err = db.GetSetting(req.Context(), "ntfy_topic")
	require.NoError(t, err)
	assert.Equal(t, "my-topic", val)
}

func TestHandleUpdateSettingsRejectsNtfyEnabledWithoutConfig(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"ntfy_enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleUpdateSettings(cfg, db, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, cfg.NtfyEnabled)
}

func TestHandleGetSettingsReturnsNtfyFields(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	cfg.NtfyEnabled = true
	cfg.NtfyTopic = "test-topic"
	cfg.NtfyToken = "secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handleGetSettings(cfg, db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, true, result["ntfy_enabled"])
	assert.Equal(t, "test-topic", result["ntfy_topic"])
	assert.Equal(t, true, result["ntfy_token_set"])
}

func TestHandleGetSettingsReturnsNullCacheRefreshedAtWhenNeverRefreshed(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handleGetSettings(config.DefaultConfig(), db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	refreshedAt, present := result["afl_show_cache_refreshed_at"]
	require.True(t, present, "afl_show_cache_refreshed_at should be present")
	assert.Nil(t, refreshedAt, "a never-refreshed cache must not serialise Go's zero time")
	assert.EqualValues(t, 0, result["afl_show_cache_count"])
}

func TestHandleGetSettingsReturnsCacheRefreshedAtAfterRefresh(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	refreshed := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	_, err = db.ReplaceCachedAFLShows(t.Context(), []models.AFLShow{
		{Title: "Naruto", Slug: "naruto", URL: "https://example.com/naruto"},
	}, refreshed)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handleGetSettings(config.DefaultConfig(), db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "2026-10-29T12:00:00Z", result["afl_show_cache_refreshed_at"])
	assert.EqualValues(t, 1, result["afl_show_cache_count"])
}

func TestHandleCreateMappingRejectsInvalidFillerAction(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"sonarr_series_id":1,"afl_slug":"naruto","filler_action":"delete"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleCreateMapping(cfg, db, nil, nil, context.Background(), &sync.WaitGroup{})(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
