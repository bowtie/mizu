package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/auth"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

func TestBackupExportImportRoundtrip(t *testing.T) {
	source, err := database.New(filepath.Join(t.TempDir(), "source.db"))
	require.NoError(t, err)
	defer source.Close()

	ctx := t.Context()
	mapping := &models.ShowMapping{
		SonarrSeriesID:  42,
		SonarrTitle:     "Detective Conan",
		AFLSlug:         "detective-conan",
		AFLTitle:        "Detective Conan",
		AutoMatched:     true,
		MatchConfidence: 0.95,
		FillerAction:    models.ActionUnmonitor,
		Enabled:         true,
		StartMode:       models.StartModeCurrentProgress,
		WatermarkMode:   models.WatermarkModeAuto,
		SyncWatermark:   1116,
	}
	require.NoError(t, source.UpsertMapping(ctx, mapping))
	require.NoError(t, source.SetSetting(ctx, "sonarr_url", "http://sonarr:8989"))
	require.NoError(t, source.SetSetting(ctx, auth.SettingPasswordHash, "$2a$10$dummyhash"))

	// Export
	req := httptest.NewRequest(http.MethodGet, "/api/backup/export", nil)
	rec := httptest.NewRecorder()
	handleExportConfig(source)(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")

	var file backupFile
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &file))
	assert.Equal(t, backupFormatVersion, file.Version)
	require.Len(t, file.Mappings, 1)
	assert.Equal(t, 42, file.Mappings[0].SonarrSeriesID)
	assert.True(t, file.Mappings[0].AutoMatched, "backup must preserve fields hidden from the public JSON")
	assert.InDelta(t, 0.95, file.Mappings[0].MatchConfidence, 0.0001)
	assert.Equal(t, "http://sonarr:8989", file.Settings["sonarr_url"])
	assert.NotContains(t, file.Settings, auth.SettingPasswordHash, "the password hash must never be exported")

	// Import into a fresh database
	target, err := database.New(filepath.Join(t.TempDir(), "target.db"))
	require.NoError(t, err)
	defer target.Close()
	cfg := config.DefaultConfig()
	target.SetSetting(ctx, auth.SettingPasswordHash, "$2a$10$targethash") // must survive import

	importReq := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(rec.Body.String()))
	importRec := httptest.NewRecorder()
	handleImportConfig(cfg, target, newSchedulerController(cfg.ScanInterval), nil, nil)(importRec, importReq)
	require.Equal(t, http.StatusOK, importRec.Code)

	restored, err := target.GetMappingBySonarrID(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, "detective-conan", restored.AFLSlug)
	assert.Equal(t, 1116, restored.SyncWatermark)

	value, err := target.GetSetting(ctx, "sonarr_url")
	require.NoError(t, err)
	assert.Equal(t, "http://sonarr:8989", value)

	hash, err := target.GetSetting(ctx, auth.SettingPasswordHash)
	require.NoError(t, err)
	assert.Equal(t, "$2a$10$targethash", hash, "import must not touch the password hash")
}

func TestImportRejectsPasswordHashOverride(t *testing.T) {
	target, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer target.Close()

	payload := `{"version":1,"settings":{"sonarr_url":"http://x","auth_password_hash":"$2a$10$evil"},"mappings":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handleImportConfig(config.DefaultConfig(), target, newSchedulerController(0), nil, nil)(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	hash, err := target.GetSetting(t.Context(), auth.SettingPasswordHash)
	require.NoError(t, err)
	assert.Empty(t, hash, "auth_password_hash in the import file must be filtered out")
}

func TestImportRejectsInvalidMapping(t *testing.T) {
	target, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer target.Close()

	tests := []struct {
		name    string
		payload string
	}{
		{"missing series id", `{"version":1,"mappings":[{"afl_slug":"naruto"}]}`},
		{"bad slug", `{"version":1,"mappings":[{"sonarr_series_id":1,"afl_slug":"bad slug!"}]}`},
		{"bad action", `{"version":1,"mappings":[{"sonarr_series_id":1,"afl_slug":"naruto","filler_action":"delete"}]}`},
		{"future version", `{"version":99,"mappings":[]}`},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(tt.payload))
		rec := httptest.NewRecorder()
		handleImportConfig(config.DefaultConfig(), target, newSchedulerController(0), nil, nil)(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, tt.name)
	}
}

// Imported settings get the same validation as PUT /api/settings, and a
// rejected import writes nothing — not even the valid mappings in it.
func TestImportRejectsInvalidSettingsAndWritesNothing(t *testing.T) {
	target, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer target.Close()

	mapping := `"mappings":[{"sonarr_series_id":1,"sonarr_title":"Naruto","afl_slug":"naruto","afl_title":"Naruto"}]`
	for name, settings := range map[string]string{
		"bad action":       `{"default_filler_action":"delete"}`,
		"bad interval":     `{"sync_interval":"5m"}`,
		"bad monitor mode": `{"sonarr_add_monitor_mode":"everything"}`,
		"bad bool":         `{"ntfy_enabled":"yes"}`,
		"bad url":          `{"sonarr_url":"ftp://sonarr"}`,
		"bad source":       `{"artwork_default_source":"nope"}`,
		"bad start mode":   `{}`,
	} {
		payload := `{"version":1,"settings":` + settings + `,` + mapping + `}`
		if name == "bad start mode" {
			payload = `{"version":1,"mappings":[{"sonarr_series_id":1,"afl_slug":"naruto","start_mode":"sideways"}]}`
		}
		req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		handleImportConfig(config.DefaultConfig(), target, newSchedulerController(0), nil, nil)(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}

	mappings, err := target.GetAllMappings(context.Background())
	require.NoError(t, err)
	assert.Empty(t, mappings, "a rejected import must not write any mapping")
	all, err := target.GetAllSettings(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, all, "default_filler_action")
}

// A valid import normalizes sync_interval ("360m" → "6h") before storing it.
func TestImportNormalizesSyncInterval(t *testing.T) {
	target, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer target.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(`{"version":1,"settings":{"sync_interval":"360m"},"mappings":[]}`))
	rec := httptest.NewRecorder()
	cfg := config.DefaultConfig()
	handleImportConfig(cfg, target, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	v, err := target.GetSetting(context.Background(), "sync_interval")
	require.NoError(t, err)
	assert.Equal(t, "6h", v)
}
