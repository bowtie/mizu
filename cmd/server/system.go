package main

import (
	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"net/http"
	"strconv"
)

func handleGetStats(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		canon, mixed, animeCanon, filler, total, err := db.GetFillerStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"canon": canon, "mixed": mixed, "anime_canon": animeCanon, "filler": filler, "total": total,
		})
	}
}

func handleGetLogs(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		offset := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}
		logs, err := db.GetSyncLogsPage(r.Context(), limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}
}

// handleBootstrap returns shows + settings + artwork providers in a
// single response, eliminating 2 extra round-trips on app mount.
func handleBootstrap(cfg *config.Config, db *database.DB, mgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Settings
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
			"artwork_default_source":        cfg.ArtworkDefaultSource,
			"tmdb_api_key_masked":           maskedTmdbKey,
			"tmdb_api_key_set":              cfg.TMDBAPIKey != "",
			"fanart_api_key_masked":         maskedFanartKey,
			"fanart_api_key_set":            cfg.FanartAPIKey != "",
			"ui_sounds_enabled":             cfg.UISoundsEnabled,
		}
		cfg.RUnlock()

		cacheCount, cacheRefreshedAt, err := db.GetAFLShowCacheStatus(r.Context())
		if err == nil {
			settings["afl_show_cache_refreshed_at"] = cacheRefreshedAt
			settings["afl_show_cache_count"] = cacheCount
		}

		// Mappings
		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// stale-while-revalidate lets the browser serve the cached
		// bootstrap response instantly on repeat visits while
		// revalidating in the background. This makes repeat page loads
		// nearly instant — the store gets data from the browser cache
		// before the server even responds.
		w.Header().Set("Cache-Control", "max-age=0, stale-while-revalidate=300")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"shows":     mappings,
			"settings":  settings,
			"providers": mgr.Providers(),
		})
	}
}
