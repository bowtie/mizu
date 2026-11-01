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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"
)

// --- Stats handler ---

func TestHandleGetStatsReturnsZeroCountsOnEmptyDB(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	handleGetStats(db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result map[string]int
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, 0, result["total"])
}

// --- Mappings CRUD ---

func TestHandleGetMappingsReturnsEmptyArrayOnEmptyDB(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/mappings", nil)
	rec := httptest.NewRecorder()

	handleGetMappings(db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result []models.ShowMapping
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Empty(t, result)
}

func TestHandleGetMappingReturns404ForMissingID(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/mappings/abc", nil)
	rec := httptest.NewRecorder()

	handleGetMapping(db)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateMappingRejectsMissingSonarrSeriesID(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"afl_slug":"naruto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleCreateMapping(cfg, db, nil, nil, context.Background(), &sync.WaitGroup{})(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateMappingRejectsMissingAFLSlug(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{"sonarr_series_id":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleCreateMapping(cfg, db, nil, nil, context.Background(), &sync.WaitGroup{})(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateMappingRejectsInvalidJSON(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	body := `{not json}`
	req := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleCreateMapping(cfg, db, nil, nil, context.Background(), &sync.WaitGroup{})(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateMappingConflictsWithExistingMapping(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	mapping := &models.ShowMapping{SonarrSeriesID: 7, AFLSlug: "naruto", AFLTitle: "Naruto"}
	require.NoError(t, db.UpsertMapping(context.Background(), mapping))

	// Duplicate create without overwrite → 409, existing mapping untouched.
	req := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(`{"sonarr_series_id":7,"sonarr_title":"Naruto","afl_slug":"one-piece"}`))
	rec := httptest.NewRecorder()
	handleCreateMapping(cfg, db, nil, nil, context.Background(), &sync.WaitGroup{})(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)

	kept, err := db.GetMappingBySonarrID(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "naruto", kept.AFLSlug)

	// overwrite=true proceeds and replaces the mapping.
	req2 := httptest.NewRequest(http.MethodPost, "/api/mappings", strings.NewReader(`{"sonarr_series_id":7,"sonarr_title":"Naruto","afl_slug":"one-piece","overwrite":true}`))
	rec2 := httptest.NewRecorder()
	handleCreateMapping(cfg, db, nil, nil, context.Background(), &sync.WaitGroup{})(rec2, req2)
	assert.Equal(t, http.StatusCreated, rec2.Code)

	kept, err = db.GetMappingBySonarrID(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "one-piece", kept.AFLSlug)
}

// --- Logs handler ---

func TestHandleGetLogsReturnsEmptyOnEmptyDB(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()

	handleGetLogs(db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result struct {
		Items []models.SyncLog `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Empty(t, result.Items)
}

func TestHandleGetLogsRejectsNegativeOffset(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/logs?offset=-5", nil)
	rec := httptest.NewRecorder()

	handleGetLogs(db)(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Artwork refresh status handler ---

func TestHandleArtworkRefreshStatusReturnsFalseWhenIdle(t *testing.T) {
	artworkRefreshing.Store(false)

	req := httptest.NewRequest(http.MethodGet, "/api/artwork/refresh-status", nil)
	rec := httptest.NewRecorder()

	handleArtworkRefreshStatus(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result map[string]bool
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.False(t, result["refreshing"])
}

// --- Sync trigger handler ---

func TestHandleTriggerSyncReturns409WhenAlreadyRunning(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	cfg := config.DefaultConfig()
	engine := syncengine.New(cfg, db)

	engine.SetRunning(true)
	defer engine.SetRunning(false)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", nil)
	rec := httptest.NewRecorder()

	handleTriggerSync(engine, nil, nil, nil)(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

// --- Artwork source handler ---

func TestHandleSetArtworkSourceRejectsInvalidSource(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	body := `{not json}`
	req := httptest.NewRequest(http.MethodPut, "/api/mappings/1/artwork/source", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleSetArtworkSource(db, nil)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
