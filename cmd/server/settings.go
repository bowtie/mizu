package main

import (
	"encoding/json"
	"fmt"
	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/netguard"
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
			"afl_refresh_interval":          cfg.AFLRefreshInterval,
			"hold_unclassified":             cfg.HoldUnclassified,
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
			writeInternalError(w, err, "")
			return
		}
		// A zero time means the cache has never been refreshed; send null
		// rather than "0001-01-01T00:00:00Z" so clients don't render it as
		// a real date.
		if cacheRefreshedAt.IsZero() {
			settings["afl_show_cache_refreshed_at"] = nil
		} else {
			settings["afl_show_cache_refreshed_at"] = cacheRefreshedAt
		}
		settings["afl_show_cache_count"] = cacheCount
		writeJSON(w, http.StatusOK, settings)
	}
}

func handleUpdateSettings(cfg *config.Config, db *database.DB, scheduler *schedulerController, syncEngine *sync.Engine, artworkMgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		// Parse every field before mutating any config, so a later invalid
		// field can't leave the running config half-updated.
		update := make(map[string]string, len(body))
		for name, raw := range body {
			k, ok := settingByName[name]
			if !ok || string(raw) == "null" {
				continue
			}
			text, err := settingText(raw)
			if err == nil {
				text, err = k.parse(text)
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid "+name+": "+err.Error())
				return
			}
			update[name] = text
		}

		cfg.RLock()
		ntfyEnabled := cfg.NtfyEnabled
		ntfyServerURL := cfg.NtfyServerURL
		ntfyTopic := cfg.NtfyTopic
		cfg.RUnlock()
		if v, ok := update["ntfy_enabled"]; ok {
			ntfyEnabled = v == "true"
		}
		if v, ok := update["ntfy_server_url"]; ok {
			ntfyServerURL = v
		}
		if v, ok := update["ntfy_topic"]; ok {
			ntfyTopic = v
		}
		if ntfyEnabled && (ntfyServerURL == "" || ntfyTopic == "") {
			writeError(w, http.StatusBadRequest, "ntfy_server_url and ntfy_topic are required when ntfy is enabled")
			return
		}
		if src, ok := update["artwork_default_source"]; ok && src != "" && src != "sonarr" && !artworkProviderRegistered(artworkMgr, src) {
			writeError(w, http.StatusBadRequest, "invalid artwork_default_source: "+src)
			return
		}

		cfg.Lock()

		// A saved credential belongs to the server it was entered for: the
		// browser never sees it, so changing only the URL would otherwise
		// send it to whatever address was typed in. Require it again
		// (checked under the lock so a concurrent save can't slip past).
		if url, ok := update["sonarr_url"]; ok && cfg.SonarrAPIKey != "" && !sameBaseURL(url, cfg.SonarrURL) && update["sonarr_api_key"] == "" {
			cfg.Unlock()
			writeError(w, http.StatusBadRequest, "enter the Sonarr API key again when changing the Sonarr URL")
			return
		}
		if _, token := update["ntfy_token"]; cfg.NtfyToken != "" && !sameBaseURL(ntfyServerURL, cfg.NtfyServerURL) && !token {
			cfg.Unlock()
			writeError(w, http.StatusBadRequest, "enter the ntfy access token again (or clear it) when changing the ntfy server")
			return
		}

		for name, v := range update {
			settingByName[name].apply(cfg, v)
		}
		if _, ok := update["sync_interval"]; ok {
			scheduler.UpdateInterval(cfg.ScanInterval)
		}

		// Snapshot values for DB persistence while still holding the lock.
		settingsSnapshot := make(map[string]string, len(settingKeys))
		for _, k := range settingKeys {
			settingsSnapshot[k.name] = k.stored(cfg)
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
			writeInternalError(w, err, "failed to persist settings")
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
		if err := netguard.ValidateServiceURL(req.URL); err != nil {
			writeError(w, http.StatusBadRequest, "Sonarr URL: "+err.Error())
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

		if serverURL != "" {
			if err := netguard.ValidateServiceURL(serverURL); err != nil {
				writeError(w, http.StatusBadRequest, "ntfy server URL: "+err.Error())
				return
			}
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

// settingText is a JSON setting value in its stored form: a string as is,
// a boolean as "true" or "false", a number as written.
func settingText(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	switch v := v.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		return strings.TrimSpace(string(raw)), nil
	default:
		return "", fmt.Errorf("not a string, boolean or number")
	}
}

// artworkProviderRegistered reports whether mgr has a provider with id.
func artworkProviderRegistered(mgr *artwork.Manager, id string) bool {
	if mgr == nil {
		return false
	}
	for _, p := range mgr.Providers() {
		if p.ID == id {
			return true
		}
	}
	return false
}
