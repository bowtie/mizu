package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mizu/internal/artwork"
	"mizu/internal/auth"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"

	"github.com/rs/zerolog/log"
)

// Config export/import: a JSON snapshot of all mappings plus the
// non-auth settings for backup and migrating instances. API keys are
// included on purpose — a restore that loses the Sonarr key isn't a
// restore — and the endpoints live in the session-protected group,
// consistent with /api/settings returning full keys.

const backupFormatVersion = 1

// importableSettings is the allowlist of app_settings keys a config
// import may write. Everything the settings UI persists qualifies, plus
// internal-but-portable keys (AFL refresh timestamp, pending manual
// matches). Deliberately excludes auth_password_hash: a crafted import
// file must never be able to replace the dashboard password.
var importableSettings = map[string]struct{}{
	"sonarr_url":                      {},
	"sonarr_api_key":                  {},
	"default_filler_action":           {},
	"default_mixed_canon_action":      {},
	"default_anime_canon_action":      {},
	"sync_interval":                   {},
	"sonarr_add_root_folder":          {},
	"sonarr_add_quality_profile_id":   {},
	"sonarr_add_monitor_mode":         {},
	"sonarr_add_season_folder":        {},
	"ntfy_enabled":                    {},
	"ntfy_server_url":                 {},
	"ntfy_topic":                      {},
	"ntfy_token":                      {},
	"ntfy_notify_on":                  {},
	"artwork_default_source":          {},
	"tmdb_api_key":                    {},
	"fanart_api_key":                  {},
	"ui_sounds_enabled":               {},
	"afl_show_cache_refreshed_at":     {},
	"manual_match_pending_series_ids": {},
}

// validAFLSlug reports whether s contains only the characters the
// scraper URL builder and mapping handlers accept.
func validAFLSlug(s string) bool {
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

// mappingBackup mirrors every show_mappings column so a backup file
// round-trips fields that models.ShowMapping hides from its JSON
// encoding (auto_matched, match_confidence).
type mappingBackup struct {
	SonarrSeriesID   int                  `json:"sonarr_series_id"`
	SonarrTitle      string               `json:"sonarr_title"`
	AFLSlug          string               `json:"afl_slug"`
	AFLTitle         string               `json:"afl_title"`
	AutoMatched      bool                 `json:"auto_matched"`
	MatchConfidence  float64              `json:"match_confidence"`
	FillerAction     models.FillerAction  `json:"filler_action"`
	MixedAction      models.FillerAction  `json:"mixed_action"`
	AnimeCanonAction models.FillerAction  `json:"anime_canon_action"`
	Enabled          bool                 `json:"enabled"`
	StartMode        models.StartMode     `json:"start_mode"`
	WatermarkMode    models.WatermarkMode `json:"watermark_mode"`
	SyncWatermark    int                  `json:"sync_watermark"`
	ArtworkSource    string               `json:"artwork_source"`
	Overview         string               `json:"overview"`
}

func toMappingBackup(m models.ShowMapping) mappingBackup {
	return mappingBackup{
		SonarrSeriesID:   m.SonarrSeriesID,
		SonarrTitle:      m.SonarrTitle,
		AFLSlug:          m.AFLSlug,
		AFLTitle:         m.AFLTitle,
		AutoMatched:      m.AutoMatched,
		MatchConfidence:  m.MatchConfidence,
		FillerAction:     m.FillerAction,
		MixedAction:      m.MixedAction,
		AnimeCanonAction: m.AnimeCanonAction,
		Enabled:          m.Enabled,
		StartMode:        m.StartMode,
		WatermarkMode:    m.WatermarkMode,
		SyncWatermark:    m.SyncWatermark,
		ArtworkSource:    m.ArtworkSource,
		Overview:         m.Overview,
	}
}

func (b mappingBackup) toMapping() models.ShowMapping {
	return models.ShowMapping{
		SonarrSeriesID:   b.SonarrSeriesID,
		SonarrTitle:      b.SonarrTitle,
		AFLSlug:          b.AFLSlug,
		AFLTitle:         b.AFLTitle,
		AutoMatched:      b.AutoMatched,
		MatchConfidence:  b.MatchConfidence,
		FillerAction:     b.FillerAction,
		MixedAction:      b.MixedAction,
		AnimeCanonAction: b.AnimeCanonAction,
		Enabled:          b.Enabled,
		StartMode:        b.StartMode,
		WatermarkMode:    b.WatermarkMode,
		SyncWatermark:    b.SyncWatermark,
		ArtworkSource:    b.ArtworkSource,
		Overview:         b.Overview,
	}
}

type backupFile struct {
	Version    int               `json:"version"`
	ExportedAt time.Time         `json:"exported_at"`
	Settings   map[string]string `json:"settings"`
	Mappings   []mappingBackup   `json:"mappings"`
}

// handleExportConfig streams the full config as a downloadable JSON file.
func handleExportConfig(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := db.GetAllSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load settings: "+err.Error())
			return
		}
		delete(settings, auth.SettingPasswordHash)

		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mappings: "+err.Error())
			return
		}
		exported := make([]mappingBackup, 0, len(mappings))
		for _, m := range mappings {
			exported = append(exported, toMappingBackup(m))
		}

		filename := "mizu-config-" + time.Now().Format("20060102") + ".json"
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		writeJSON(w, http.StatusOK, backupFile{
			Version:    backupFormatVersion,
			ExportedAt: time.Now().UTC(),
			Settings:   settings,
			Mappings:   exported,
		})
	}
}

// handleImportConfig restores a config file: allowed settings are
// upserted and mappings are merged by Sonarr series ID (an existing
// mapping is replaced — this is an explicit restore, unlike the create
// endpoint which 409s). Runtime state (scheduler interval, Sonarr/ntfy
// clients, artwork providers) is rebuilt from the imported values so a
// restore takes effect without a restart.
func handleImportConfig(cfg *config.Config, db *database.DB, scheduler *schedulerController, engine *syncengine.Engine, artworkMgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var file backupFile
		if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if file.Version > backupFormatVersion {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported backup version %d (this build supports %d)", file.Version, backupFormatVersion))
			return
		}

		for _, m := range file.Mappings {
			if m.SonarrSeriesID == 0 {
				writeError(w, http.StatusBadRequest, "mapping entry missing sonarr_series_id")
				return
			}
			if !validAFLSlug(m.AFLSlug) {
				writeError(w, http.StatusBadRequest, "invalid afl_slug: "+m.AFLSlug)
				return
			}
			for _, action := range []models.FillerAction{m.FillerAction, m.MixedAction, m.AnimeCanonAction} {
				if action != "" && !config.IsValidFillerAction(string(action)) {
					writeError(w, http.StatusBadRequest, "invalid filler action: "+string(action))
					return
				}
			}
		}

		imported := make(map[string]string, len(file.Settings))
		for k, v := range file.Settings {
			if _, ok := importableSettings[k]; ok {
				imported[k] = v
			}
		}
		if len(imported) > 0 {
			if err := db.SetSettings(r.Context(), imported); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to persist settings: "+err.Error())
				return
			}
		}

		for _, mb := range file.Mappings {
			m := mb.toMapping()
			if err := db.UpsertMapping(r.Context(), &m); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to import mapping for series %d: %s", m.SonarrSeriesID, err))
				return
			}
		}

		// Reload the in-memory config from the imported settings and
		// rebuild everything derived from it. applyRuntimeSettings takes
		// the config write lock, so this is safe against concurrent readers.
		applyRuntimeSettings(r.Context(), db, cfg)
		scheduler.UpdateInterval(cfg.ScanInterval)
		if engine != nil {
			engine.UpdateClients()
		}
		if artworkMgr != nil {
			cfg.RLock()
			tmdbKey := cfg.TMDBAPIKey
			fanartKey := cfg.FanartAPIKey
			requestTimeout := cfg.RequestTimeout
			cfg.RUnlock()
			rebuildArtworkProviders(requestTimeout, tmdbKey, fanartKey, artworkMgr)
		}

		log.Info().Int("settings", len(imported)).Int("mappings", len(file.Mappings)).Msg("config import applied")
		writeJSON(w, http.StatusOK, map[string]int{
			"settings_imported": len(imported),
			"mappings_imported": len(file.Mappings),
		})
	}
}
