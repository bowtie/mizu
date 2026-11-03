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
	handleExportConfig(source, auth.NewManager(source), newAccessControl(accessPolicy{}))(rec, req)
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

func newExportTestDB(t *testing.T) (*database.DB, *auth.Manager) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "export.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	ctx := t.Context()
	require.NoError(t, db.SetSettings(ctx, map[string]string{
		"sonarr_url":      "http://sonarr:8989",
		"sonarr_api_key":  "sonarr-secret",
		"tmdb_api_key":    "tmdb-secret",
		"fanart_api_key":  "fanart-secret",
		"ntfy_token":      "ntfy-secret",
		settingAuthMode:   string(authModePassword),
		"trusted_proxies": "10.0.0.2",
	}))
	mgr := auth.NewManager(db)
	require.NoError(t, mgr.SetPassword(ctx, "export-password"))
	return db, mgr
}

func exportWith(t *testing.T, db *database.DB, mgr *auth.Manager, method, body string) (*httptest.ResponseRecorder, backupFile) {
	t.Helper()
	req := httptest.NewRequest(method, "/api/backup/export", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handleExportConfig(db, mgr, newAccessControl(accessPolicy{}))(rec, req)
	var file backupFile
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &file))
	}
	return rec, file
}

func TestExportOmitsSecretsByDefault(t *testing.T) {
	db, mgr := newExportTestDB(t)
	for _, tc := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPost, ""},
		{http.MethodPost, `{"include_secrets":false}`},
	} {
		rec, file := exportWith(t, db, mgr, tc.method, tc.body)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, file.IncludesSecrets)
		for k := range secretSettings {
			assert.NotContains(t, file.Settings, k)
		}
		assert.NotContains(t, rec.Body.String(), "-secret")
		assert.NotContains(t, file.Settings, settingAuthMode, "sign-in mode is never exported")
		assert.NotContains(t, file.Settings, settingTrustedProxies)
		assert.Equal(t, "http://sonarr:8989", file.Settings["sonarr_url"])
	}
}

func TestExportWithSecretsNeedsPassword(t *testing.T) {
	db, mgr := newExportTestDB(t)

	rec, _ := exportWith(t, db, mgr, http.MethodPost, `{"include_secrets":true}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec, _ = exportWith(t, db, mgr, http.MethodPost, `{"include_secrets":true,"password":"wrong-password"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec, file := exportWith(t, db, mgr, http.MethodPost, `{"include_secrets":true,"password":"export-password"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, file.IncludesSecrets)
	assert.Equal(t, "sonarr-secret", file.Settings["sonarr_api_key"])
	assert.Equal(t, "ntfy-secret", file.Settings["ntfy_token"])
}

func TestImportClearsCredentialForNewURL(t *testing.T) {
	target, err := database.New(filepath.Join(t.TempDir(), "target.db"))
	require.NoError(t, err)
	defer target.Close()
	ctx := t.Context()
	require.NoError(t, target.SetSettings(ctx, map[string]string{"sonarr_url": "http://sonarr:8989", "sonarr_api_key": "keep-me-safe"}))
	cfg := config.DefaultConfig()
	applyRuntimeSettings(ctx, target, cfg)

	run := func(payload string) {
		req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		handleImportConfig(cfg, target, newSchedulerController(cfg.ScanInterval), nil, nil)(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	// Same URL, no key in the file: the saved key stays.
	run(`{"version":1,"settings":{"sonarr_url":"http://sonarr:8989/"},"mappings":[]}`)
	key, _ := target.GetSetting(ctx, "sonarr_api_key")
	assert.Equal(t, "keep-me-safe", key)

	// A different URL without a key must not inherit the saved one.
	run(`{"version":1,"settings":{"sonarr_url":"http://elsewhere:8989"},"mappings":[]}`)
	key, _ = target.GetSetting(ctx, "sonarr_api_key")
	assert.Empty(t, key)
	cfg.RLock()
	assert.Empty(t, cfg.SonarrAPIKey)
	cfg.RUnlock()
}

func TestImportRejectsSmugglingURL(t *testing.T) {
	target, err := database.New(filepath.Join(t.TempDir(), "target.db"))
	require.NoError(t, err)
	defer target.Close()
	payload := `{"version":1,"settings":{"sonarr_url":"http://169.254.169.254/latest/meta-data/?"},"mappings":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handleImportConfig(config.DefaultConfig(), target, newSchedulerController(0), nil, nil)(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
