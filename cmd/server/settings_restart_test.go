package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
)

// Every setting saved in Settings reads back the same after Mizu restarts.
func TestSavedSettingsSurviveARestart(t *testing.T) {
	for _, body := range []string{
		`{"sonarr_url":"http://192.168.1.10:8989"}`,
		`{"sonarr_url":""}`,
		`{"sonarr_api_key":"sonarr-key"}`,
		`{"default_filler_action":"monitor"}`,
		`{"default_mixed_canon_action":"unmonitor"}`,
		`{"default_anime_canon_action":"unmonitor"}`,
		`{"sync_interval":"24h"}`,
		`{"sonarr_add_root_folder":"/tv"}`,
		`{"sonarr_add_quality_profile_id":3}`,
		`{"sonarr_add_monitor_mode":"future"}`,
		`{"sonarr_add_monitor_mode":""}`,
		`{"sonarr_add_season_folder":false}`,
		`{"sonarr_add_season_folder":true}`,
		`{"ntfy_enabled":true,"ntfy_server_url":"https://ntfy.example.com","ntfy_topic":"mizu","ntfy_token":"tok"}`,
		`{"ntfy_server_url":""}`,
		`{"ntfy_notify_on":"changes"}`,
		`{"ntfy_notify_new_titles":true}`,
		`{"ntfy_notify_new_titles":false}`,
		`{"afl_refresh_interval":"30d"}`,
		`{"hold_unclassified":"listed"}`,
		`{"artwork_default_source":""}`,
		`{"tmdb_api_key":"tmdb-key"}`,
		`{"fanart_api_key":"fanart-key"}`,
		`{"ui_sounds_enabled":false}`,
		`{"ui_sounds_enabled":true}`,
	} {
		t.Run(body, func(t *testing.T) {
			db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			running := config.DefaultConfig()
			rec := httptest.NewRecorder()
			handleUpdateSettings(running, db, newSchedulerController(running.ScanInterval), nil, nil)(rec, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body)))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			restarted := config.DefaultConfig()
			applyRuntimeSettings(t.Context(), db, restarted)

			assert.Equal(t, settingsJSON(t, running, db), settingsJSON(t, restarted, db))
		})
	}
}

func settingsJSON(t *testing.T, cfg *config.Config, db *database.DB) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	handleGetSettings(cfg, db)(rec, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}
