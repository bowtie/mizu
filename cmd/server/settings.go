package main

import (
	"encoding/json"
	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/notifier"
	"mizu/internal/sonarr"
	"mizu/internal/sync"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func handleGetSettings(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg.RLock()
		maskedSonarrKey := maskKey(cfg.SonarrAPIKey)
		maskedTmdbKey := maskKey(cfg.TMDBAPIKey)
		maskedFanartKey := maskKey(cfg.FanartAPIKey)
		settings := map[string]interface{}{
			"sonarr_url":                    cfg.SonarrURL,
			"sonarr_api_key_masked":         maskedSonarrKey,
			"sonarr_api_key_set":            cfg.SonarrAPIKey != "",
			"default_filler_action":         string(cfg.DefaultFillerAction),
			"default_mixed_canon_action":    string(cfg.DefaultMixedCanonAction),
			"default_anime_canon_action":    string(cfg.DefaultAnimeCanonAction),
			"sync_interval":                 scanIntervalToValue(cfg.ScanInterval),
			"sonarr_add_root_folder":        cfg.SonarrAddRootFolder,
			"sonarr_add_quality_profile_id": cfg.SonarrAddQualityProfileID,
			"sonarr_add_monitor_mode":       cfg.SonarrAddMonitorMode,
			"sonarr_add_season_folder":      cfg.SonarrAddSeasonFolder,
			"ntfy_enabled":                  cfg.NtfyEnabled,
			"ntfy_server_url":               cfg.NtfyServerURL,
			"ntfy_topic":                    cfg.NtfyTopic,
			"ntfy_token_set":                cfg.NtfyToken != "",
			"ntfy_notify_on":                cfg.NtfyNotifyOn,
			"ntfy_notify_new_titles":        cfg.NtfyNotifyNewTitles,
			"artwork_default_source":        cfg.ArtworkDefaultSource,
			"tmdb_api_key_masked":           maskedTmdbKey,
			"tmdb_api_key_set":              cfg.TMDBAPIKey != "",
			"fanart_api_key_masked":         maskedFanartKey,
			"fanart_api_key_set":            cfg.FanartAPIKey != "",
			"ui_sounds_enabled":             cfg.UISoundsEnabled,
		}
		cfg.RUnlock()

		cacheCount, cacheRefreshedAt, err := db.GetAFLShowCacheStatus(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		settings["afl_show_cache_refreshed_at"] = cacheRefreshedAt
		settings["afl_show_cache_count"] = cacheCount
		writeJSON(w, http.StatusOK, settings)
	}
}

func handleUpdateSettings(cfg *config.Config, db *database.DB, scheduler *schedulerController, syncEngine *sync.Engine, artworkMgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var update struct {
			SonarrURL                 *string `json:"sonarr_url"`
			SonarrAPIKey              *string `json:"sonarr_api_key"`
			DefaultFillerAction       *string `json:"default_filler_action"`
			DefaultMixedCanonAction   *string `json:"default_mixed_canon_action"`
			DefaultAnimeCanonAction   *string `json:"default_anime_canon_action"`
			SyncInterval              *string `json:"sync_interval"`
			SonarrAddRootFolder       *string `json:"sonarr_add_root_folder"`
			SonarrAddQualityProfileID *int    `json:"sonarr_add_quality_profile_id"`
			SonarrAddMonitorMode      *string `json:"sonarr_add_monitor_mode"`
			SonarrAddSeasonFolder     *bool   `json:"sonarr_add_season_folder"`
			NtfyEnabled               *bool   `json:"ntfy_enabled"`
			NtfyServerURL             *string `json:"ntfy_server_url"`
			NtfyTopic                 *string `json:"ntfy_topic"`
			NtfyToken                 *string `json:"ntfy_token"`
			NtfyNotifyOn              *string `json:"ntfy_notify_on"`
			NtfyNotifyNewTitles       *bool   `json:"ntfy_notify_new_titles"`
			ArtworkDefaultSource      *string `json:"artwork_default_source"`
			TMDBAPIKey                *string `json:"tmdb_api_key"`
			FanartAPIKey              *string `json:"fanart_api_key"`
			UISoundsEnabled           *bool   `json:"ui_sounds_enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		// Validate all fields before mutating any config. This prevents
		// partial in-memory mutations when a later field fails validation,
		// which would leave the running config out of sync with the DB.
		if update.DefaultFillerAction != nil && !config.IsValidFillerAction(*update.DefaultFillerAction) {
			writeError(w, http.StatusBadRequest, "invalid default_filler_action")
			return
		}
		if update.DefaultMixedCanonAction != nil && !config.IsValidFillerAction(*update.DefaultMixedCanonAction) {
			writeError(w, http.StatusBadRequest, "invalid default_mixed_canon_action")
			return
		}
		if update.DefaultAnimeCanonAction != nil && !config.IsValidFillerAction(*update.DefaultAnimeCanonAction) {
			writeError(w, http.StatusBadRequest, "invalid default_anime_canon_action")
			return
		}
		var normalizedSyncInterval string
		var syncDuration time.Duration
		if update.SyncInterval != nil {
			var err error
			normalizedSyncInterval, syncDuration, err = normalizeScanInterval(*update.SyncInterval)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid sync interval")
				return
			}
		}
		if update.SonarrAddMonitorMode != nil && !config.IsValidSonarrMonitorMode(*update.SonarrAddMonitorMode) {
			writeError(w, http.StatusBadRequest, "invalid Sonarr monitor mode")
			return
		}
		if update.NtfyNotifyOn != nil && !config.IsValidNtfyNotifyOn(*update.NtfyNotifyOn) {
			writeError(w, http.StatusBadRequest, "invalid ntfy_notify_on")
			return
		}
		// Compute effective ntfy values to validate without mutating cfg.
		cfg.RLock()
		ntfyEnabled := cfg.NtfyEnabled
		ntfyServerURL := cfg.NtfyServerURL
		ntfyTopic := cfg.NtfyTopic
		cfg.RUnlock()
		if update.NtfyEnabled != nil {
			ntfyEnabled = *update.NtfyEnabled
		}
		if update.NtfyServerURL != nil {
			ntfyServerURL = strings.TrimRight(*update.NtfyServerURL, "/")
		}
		if update.NtfyTopic != nil {
			ntfyTopic = *update.NtfyTopic
		}
		if ntfyEnabled && (ntfyServerURL == "" || ntfyTopic == "") {
			writeError(w, http.StatusBadRequest, "ntfy_server_url and ntfy_topic are required when ntfy is enabled")
			return
		}
		if update.ArtworkDefaultSource != nil {
			src := *update.ArtworkDefaultSource
			if src != "" && src != "sonarr" {
				valid := false
				for _, p := range artworkMgr.Providers() {
					if p.ID == src {
						valid = true
						break
					}
				}
				if !valid {
					writeError(w, http.StatusBadRequest, "invalid artwork_default_source: "+src)
					return
				}
			}
		}

		// All validation passed — apply mutations under the write lock.
		cfg.Lock()

		if update.SonarrURL != nil {
			cfg.SonarrURL = strings.TrimRight(*update.SonarrURL, "/")
		}
		if update.SonarrAPIKey != nil {
			cfg.SonarrAPIKey = *update.SonarrAPIKey
		}
		if update.DefaultFillerAction != nil {
			cfg.DefaultFillerAction = config.FillerAction(*update.DefaultFillerAction)
		}
		if update.DefaultMixedCanonAction != nil {
			cfg.DefaultMixedCanonAction = config.FillerAction(*update.DefaultMixedCanonAction)
		}
		if update.DefaultAnimeCanonAction != nil {
			cfg.DefaultAnimeCanonAction = config.FillerAction(*update.DefaultAnimeCanonAction)
		}
		if update.SyncInterval != nil {
			cfg.ScanInterval = syncDuration
			scheduler.UpdateInterval(syncDuration)
			update.SyncInterval = &normalizedSyncInterval
		}
		if update.SonarrAddRootFolder != nil {
			cfg.SonarrAddRootFolder = *update.SonarrAddRootFolder
		}
		if update.SonarrAddQualityProfileID != nil {
			cfg.SonarrAddQualityProfileID = *update.SonarrAddQualityProfileID
		}
		if update.SonarrAddMonitorMode != nil {
			cfg.SonarrAddMonitorMode = *update.SonarrAddMonitorMode
		}
		if update.SonarrAddSeasonFolder != nil {
			cfg.SonarrAddSeasonFolder = *update.SonarrAddSeasonFolder
		}
		cfg.NtfyEnabled = ntfyEnabled
		cfg.NtfyServerURL = ntfyServerURL
		cfg.NtfyTopic = ntfyTopic
		if update.NtfyToken != nil {
			cfg.NtfyToken = *update.NtfyToken
		}
		if update.NtfyNotifyNewTitles != nil {
			cfg.NtfyNotifyNewTitles = *update.NtfyNotifyNewTitles
		}
		if update.NtfyNotifyOn != nil {
			cfg.NtfyNotifyOn = *update.NtfyNotifyOn
		}
		if update.ArtworkDefaultSource != nil {
			cfg.ArtworkDefaultSource = *update.ArtworkDefaultSource
		}
		if update.TMDBAPIKey != nil {
			cfg.TMDBAPIKey = *update.TMDBAPIKey
		}
		if update.FanartAPIKey != nil {
			cfg.FanartAPIKey = *update.FanartAPIKey
		}
		if update.UISoundsEnabled != nil {
			cfg.UISoundsEnabled = *update.UISoundsEnabled
		}

		// Snapshot values for DB persistence while still holding the lock.
		settingsSnapshot := map[string]string{
			"sonarr_url":                    cfg.SonarrURL,
			"sonarr_api_key":                cfg.SonarrAPIKey,
			"default_filler_action":         string(cfg.DefaultFillerAction),
			"default_mixed_canon_action":    string(cfg.DefaultMixedCanonAction),
			"default_anime_canon_action":    string(cfg.DefaultAnimeCanonAction),
			"sync_interval":                 scanIntervalToValue(cfg.ScanInterval),
			"sonarr_add_root_folder":        cfg.SonarrAddRootFolder,
			"sonarr_add_quality_profile_id": strconv.Itoa(cfg.SonarrAddQualityProfileID),
			"sonarr_add_monitor_mode":       cfg.SonarrAddMonitorMode,
			"sonarr_add_season_folder":      strconv.FormatBool(cfg.SonarrAddSeasonFolder),
			"ntfy_enabled":                  strconv.FormatBool(cfg.NtfyEnabled),
			"ntfy_server_url":               cfg.NtfyServerURL,
			"ntfy_topic":                    cfg.NtfyTopic,
			"ntfy_token":                    cfg.NtfyToken,
			"ntfy_notify_on":                cfg.NtfyNotifyOn,
			"ntfy_notify_new_titles":        strconv.FormatBool(cfg.NtfyNotifyNewTitles),
			"artwork_default_source":        cfg.ArtworkDefaultSource,
			"tmdb_api_key":                  cfg.TMDBAPIKey,
			"fanart_api_key":                cfg.FanartAPIKey,
			"ui_sounds_enabled":             strconv.FormatBool(cfg.UISoundsEnabled),
		}
		tmdbKey := cfg.TMDBAPIKey
		fanartKey := cfg.FanartAPIKey
		requestTimeout := cfg.RequestTimeout
		cfg.Unlock()

		// Persist to DB outside the config lock so concurrent readers aren't blocked.
		// All settings are written in a single transaction — a partial failure
		// can't leave the DB with a mix of old and new values. Report the failure
		// so the UI doesn't show a false "Saved" when the DB write didn't stick.
		if err := db.SetSettings(r.Context(), settingsSnapshot); err != nil {
			log.Error().Err(err).Msg("failed to persist settings")
			writeError(w, http.StatusInternalServerError, "failed to persist settings: "+err.Error())
			return
		}

		// Recreate Sonarr and ntfy clients with updated config
		if syncEngine != nil {
			syncEngine.UpdateClients()
		}

		// Recreate artwork providers so new API keys take effect immediately.
		if artworkMgr != nil {
			rebuildArtworkProviders(requestTimeout, tmdbKey, fanartKey, artworkMgr)
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "settings updated"})
	}
}

// rebuildArtworkProviders recreates the artwork provider set from the
// current config values. Shared by settings updates and config import.
func rebuildArtworkProviders(requestTimeout time.Duration, tmdbKey, fanartKey string, artworkMgr *artwork.Manager) {
	artworkMgr.UpdateProviders([]artwork.Provider{
		artwork.NewAniListProvider(requestTimeout),
		artwork.NewTMDBProvider(tmdbKey, requestTimeout),
		artwork.NewFanartProvider(fanartKey, requestTimeout),
		artwork.NewKitsuProvider(requestTimeout),
	})
}

// handleTestSonarr checks a Sonarr URL/key pair. The stored key is never
// sent to the browser, so an empty api_key means "use the saved one" — but
// only against the saved URL, so a request can't ship the key elsewhere.
func handleTestSonarr(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL    string `json:"url"`
			APIKey string `json:"api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.APIKey == "" {
			cfg.RLock()
			savedURL, savedKey := cfg.SonarrURL, cfg.SonarrAPIKey
			cfg.RUnlock()
			if savedKey != "" && !sameBaseURL(req.URL, savedURL) {
				writeError(w, http.StatusBadRequest, "enter the API key to test a different Sonarr URL")
				return
			}
			req.APIKey = savedKey
		}
		if req.URL == "" || req.APIKey == "" {
			writeError(w, http.StatusBadRequest, "url and api_key are required")
			return
		}
		c := sonarr.New(strings.TrimRight(req.URL, "/"), req.APIKey, 10*time.Second)
		info, err := c.TestConnection(r.Context())
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": humanizeError(err)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "version": info})
	}
}

func handleTestNtfy(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServerURL *string `json:"server_url"`
			Topic     *string `json:"topic"`
			Token     *string `json:"token"`
		}
		// Body is optional; if provided, use those values for a one-off test.
		// Otherwise, test with the currently configured settings.
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
		}

		cfg.RLock()
		serverURL := cfg.NtfyServerURL
		topic := cfg.NtfyTopic
		token := cfg.NtfyToken
		cfg.RUnlock()
		if req.ServerURL != nil {
			// The saved token belongs to the saved server; never send it
			// to a different one.
			if !sameBaseURL(*req.ServerURL, serverURL) {
				token = ""
			}
			serverURL = strings.TrimRight(*req.ServerURL, "/")
		}
		if req.Topic != nil {
			topic = *req.Topic
		}
		if req.Token != nil {
			token = *req.Token
		}

		n := notifier.New(serverURL, topic, token)
		if !n.Enabled() {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": "ntfy is not configured (missing server URL or topic)"})
			return
		}
		if err := n.SendTestNotification(r.Context()); err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": humanizeError(err)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	}
}

// sameBaseURL compares two service base URLs, ignoring case and trailing
// slashes.
func sameBaseURL(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(a), "/"), strings.TrimRight(strings.TrimSpace(b), "/"))
}
