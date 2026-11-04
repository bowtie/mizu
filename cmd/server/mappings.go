package main

import (
	"context"
	"encoding/json"
	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	"mizu/internal/sync"
	"net/http"
	"strconv"
	stdsync "sync"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

func handleGetMappings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, mappings)
	}
}

func handleGetMapping(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		if mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}
		writeJSON(w, http.StatusOK, mapping)
	}
}

func actionOrDefault(value *string, fallback config.FillerAction) models.FillerAction {
	if value == nil || *value == "" {
		return models.FillerAction(fallback)
	}
	return models.FillerAction(*value)
}

func startModeOrDefault(value *string) models.StartMode {
	if value == nil || *value == "" {
		return models.StartModeCurrentProgress
	}
	if models.StartMode(*value) == models.StartModeFromBeginning {
		return models.StartModeFromBeginning
	}
	return models.StartModeCurrentProgress
}

func watermarkModeOrDefault(value *string, startMode models.StartMode) models.WatermarkMode {
	if value != nil {
		if models.WatermarkMode(*value) == models.WatermarkModeManual {
			return models.WatermarkModeManual
		}
		if models.WatermarkMode(*value) == models.WatermarkModeAuto {
			return models.WatermarkModeAuto
		}
	}
	if startMode == models.StartModeFromBeginning {
		return models.WatermarkModeManual
	}
	return models.WatermarkModeAuto
}

func clampWatermark(value *int) int {
	if value == nil || *value < 0 {
		return 0
	}
	return *value
}

func handleCreateMapping(cfg *config.Config, db *database.DB, syncEngine *sync.Engine, artworkMgr *artwork.Manager, appCtx context.Context, bgWG *stdsync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SonarrSeriesID   int     `json:"sonarr_series_id"`
			SonarrTitle      string  `json:"sonarr_title"`
			AFLSlug          string  `json:"afl_slug"`
			AFLTitle         string  `json:"afl_title"`
			AutoMatch        bool    `json:"auto_match"`
			FillerAction     *string `json:"filler_action"`
			MixedAction      *string `json:"mixed_action"`
			AnimeCanonAction *string `json:"anime_canon_action"`
			Enabled          *bool   `json:"enabled"`
			StartMode        *string `json:"start_mode"`
			WatermarkMode    *string `json:"watermark_mode"`
			SyncWatermark    *int    `json:"sync_watermark"`
			Overwrite        bool    `json:"overwrite"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.SonarrSeriesID == 0 {
			writeError(w, http.StatusBadRequest, "sonarr_series_id is required")
			return
		}

		// Refuse to silently overwrite: UpsertMapping is an update on
		// conflict, so a double-submit or stale tab would otherwise reset
		// another session's per-show config. Clients confirm with
		// overwrite=true to proceed.
		if !req.Overwrite {
			if existing, err := db.GetMappingBySonarrID(r.Context(), req.SonarrSeriesID); err == nil && existing != nil {
				writeError(w, http.StatusConflict, "a mapping for this series already exists")
				return
			}
		}

		if req.AutoMatch && req.AFLSlug == "" {
			if syncEngine == nil {
				writeError(w, http.StatusInternalServerError, "auto-match unavailable")
				return
			}
			// Check AFL cache freshness before attempting auto-match.
			// If the cache is empty or stale, AutoMatchSeries would
			// synchronously scrape the entire AFL website, likely
			// exceeding the HTTP timeout. Return a clear error instead.
			cacheCount, _, cacheErr := db.GetAFLShowCacheStatus(r.Context())
			if cacheErr != nil || cacheCount == 0 {
				writeError(w, http.StatusConflict, "AFL cache is empty: refresh it before using auto-match")
				return
			}
			mapping, err := syncEngine.AutoMatchSeries(r.Context(), req.SonarrSeriesID)
			if err != nil {
				writeError(w, http.StatusBadGateway, "auto-match failed: "+err.Error())
				return
			}
			if mapping == nil {
				writeError(w, http.StatusNotFound, "no AFL match candidates found")
				return
			}
			storedMapping, err := db.GetMappingBySonarrID(r.Context(), req.SonarrSeriesID)
			if err != nil || storedMapping == nil {
				writeError(w, http.StatusInternalServerError, "failed to load saved auto-match")
				return
			}
			hydratedMapping, err := syncEngine.HydrateMapping(r.Context(), storedMapping.ID)
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to hydrate mapping episodes: "+err.Error())
				return
			}
			if hydratedMapping != nil {
				storedMapping = hydratedMapping
			}
			if err := clearManualMatchPendingSeriesID(r.Context(), db, req.SonarrSeriesID); err != nil {
				log.Warn().Err(err).Int("sonarr_series_id", req.SonarrSeriesID).Msg("failed to clear manual-match pending series")
			}
			triggerArtworkFetch(appCtx, bgWG, artworkMgr, cfg, syncEngine, storedMapping)
			writeJSON(w, http.StatusCreated, storedMapping)
			return
		}

		if req.AFLSlug == "" {
			writeError(w, http.StatusBadRequest, "afl_slug is required for manual mapping")
			return
		}

		if req.FillerAction != nil && !config.IsValidFillerAction(*req.FillerAction) {
			writeError(w, http.StatusBadRequest, "invalid filler_action")
			return
		}
		if req.MixedAction != nil && !config.IsValidFillerAction(*req.MixedAction) {
			writeError(w, http.StatusBadRequest, "invalid mixed_action")
			return
		}
		if req.AnimeCanonAction != nil && !config.IsValidFillerAction(*req.AnimeCanonAction) {
			writeError(w, http.StatusBadRequest, "invalid anime_canon_action")
			return
		}

		sonarrTitle := req.SonarrTitle
		var sonarrOverview string
		if sonarrTitle == "" && syncEngine != nil {
			sonarrSeries, err := syncEngine.SonarrClient().GetSeries(r.Context(), req.SonarrSeriesID)
			if err == nil && sonarrSeries != nil {
				sonarrTitle = sonarrSeries.Title
				sonarrOverview = sonarrSeries.Overview
			}
		}

		startMode := startModeOrDefault(req.StartMode)
		initialWatermark := 0
		if syncEngine != nil {
			watermark, err := syncEngine.InitialWatermarkForStartMode(r.Context(), req.SonarrSeriesID, req.AFLSlug, startMode)
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to compute the starting watermark: "+err.Error())
				return
			}
			initialWatermark = watermark
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		watermarkMode := watermarkModeOrDefault(req.WatermarkMode, startMode)
		if req.SyncWatermark != nil {
			initialWatermark = clampWatermark(req.SyncWatermark)
			watermarkMode = models.WatermarkModeManual
		}

		cfg.RLock()
		defaultFiller := cfg.DefaultFillerAction
		defaultMixed := cfg.DefaultMixedCanonAction
		defaultAnimeCanon := cfg.DefaultAnimeCanonAction
		cfg.RUnlock()
		mapping := models.ShowMapping{
			SonarrSeriesID:   req.SonarrSeriesID,
			SonarrTitle:      sonarrTitle,
			Overview:         sonarrOverview,
			AFLSlug:          req.AFLSlug,
			AFLTitle:         req.AFLTitle,
			AutoMatched:      false,
			MatchConfidence:  1.0,
			FillerAction:     actionOrDefault(req.FillerAction, defaultFiller),
			MixedAction:      actionOrDefault(req.MixedAction, defaultMixed),
			AnimeCanonAction: actionOrDefault(req.AnimeCanonAction, defaultAnimeCanon),
			Enabled:          enabled,
			StartMode:        startMode,
			WatermarkMode:    watermarkMode,
			SyncWatermark:    initialWatermark,
		}
		if err := db.UpsertMapping(r.Context(), &mapping); err != nil {
			writeInternalError(w, err, "failed to save mapping")
			return
		}

		storedMapping, err := db.GetMappingBySonarrID(r.Context(), req.SonarrSeriesID)
		if err != nil || storedMapping == nil {
			writeError(w, http.StatusInternalServerError, "failed to load saved mapping")
			return
		}

		if syncEngine != nil {
			hydratedMapping, err := syncEngine.HydrateMapping(r.Context(), storedMapping.ID)
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to hydrate mapping episodes: "+err.Error())
				return
			}
			if hydratedMapping != nil {
				storedMapping = hydratedMapping
			}
		}

		if err := clearManualMatchPendingSeriesID(r.Context(), db, req.SonarrSeriesID); err != nil {
			log.Warn().Err(err).Int("sonarr_series_id", req.SonarrSeriesID).Msg("failed to clear manual-match pending series")
		}

		triggerArtworkFetch(appCtx, bgWG, artworkMgr, cfg, syncEngine, storedMapping)
		writeJSON(w, http.StatusCreated, storedMapping)
	}
}

// triggerArtworkFetch starts a background artwork fetch for a newly created
// mapping. It uses the app context (not the request context) so the fetch
// survives after the HTTP response is sent. If the default source is "sonarr"
// there's nothing to download — the frontend uses Sonarr URLs directly — so we
// skip the fetch in that case.
func triggerArtworkFetch(appCtx context.Context, bgWG *stdsync.WaitGroup, mgr *artwork.Manager, cfg *config.Config, syncEngine *sync.Engine, mapping *models.ShowMapping) {
	if mgr == nil || mapping == nil {
		return
	}
	cfg.RLock()
	artworkDefaultSource := cfg.ArtworkDefaultSource
	cfg.RUnlock()
	source := mgr.ResolveSource(mapping.ArtworkSource, artworkDefaultSource)
	if source == "sonarr" || source == "" {
		return
	}

	bgWG.Add(1)
	go func() {
		defer bgWG.Done()
		series := models.SonarrSeries{
			ID:    mapping.SonarrSeriesID,
			Title: mapping.SonarrTitle,
		}
		if syncEngine != nil {
			if fetched, err := syncEngine.SonarrClient().GetSeries(appCtx, mapping.SonarrSeriesID); err == nil && fetched != nil {
				series = *fetched
			} else if err != nil {
				log.Warn().Err(err).Int("sonarr_series_id", mapping.SonarrSeriesID).Msg("failed to fetch Sonarr series for artwork-on-create")
			}
		}
		if mapping.AFLSlug != "" {
			series.AlternateTitles = append(series.AlternateTitles, models.SonarrAltTitle{Title: mapping.AFLSlug})
		}
		result, err := mgr.FetchAndStore(appCtx, mapping.ID, series, source)
		if err != nil {
			log.Warn().Err(err).Int64("mapping_id", mapping.ID).Msg("artwork fetch on mapping create failed")
			return
		}
		if result != nil && result.Found {
			log.Info().Int64("mapping_id", mapping.ID).Str("source", result.Source).Msg("artwork fetched on mapping create")
		}
	}()
}

func handleUpdateMapping(syncEngine *sync.Engine, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		if mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}

		var update struct {
			FillerAction       *string `json:"filler_action"`
			MixedAction        *string `json:"mixed_action"`
			AnimeCanonAction   *string `json:"anime_canon_action"`
			Enabled            *bool   `json:"enabled"`
			AFLSlug            *string `json:"afl_slug"`
			StartMode          *string `json:"start_mode"`
			WatermarkMode      *string `json:"watermark_mode"`
			SyncWatermark      *int    `json:"sync_watermark"`
			NotificationsMuted *bool   `json:"notifications_muted"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if update.FillerAction != nil && !config.IsValidFillerAction(*update.FillerAction) {
			writeError(w, http.StatusBadRequest, "invalid filler_action")
			return
		}
		if update.MixedAction != nil && !config.IsValidFillerAction(*update.MixedAction) {
			writeError(w, http.StatusBadRequest, "invalid mixed_action")
			return
		}
		if update.AnimeCanonAction != nil && !config.IsValidFillerAction(*update.AnimeCanonAction) {
			writeError(w, http.StatusBadRequest, "invalid anime_canon_action")
			return
		}
		if update.FillerAction != nil {
			mapping.FillerAction = models.FillerAction(*update.FillerAction)
		}
		if update.MixedAction != nil {
			mapping.MixedAction = models.FillerAction(*update.MixedAction)
		}
		if update.AnimeCanonAction != nil {
			mapping.AnimeCanonAction = models.FillerAction(*update.AnimeCanonAction)
		}
		if update.Enabled != nil {
			mapping.Enabled = *update.Enabled
		}
		if update.NotificationsMuted != nil {
			mapping.NotificationsMuted = *update.NotificationsMuted
		}
		if update.AFLSlug != nil {
			if !validAFLSlug(*update.AFLSlug) {
				writeError(w, http.StatusBadRequest, "invalid afl_slug: only letters, numbers, hyphens, and underscores are allowed")
				return
			}
			mapping.AFLSlug = *update.AFLSlug
		}
		if update.StartMode != nil {
			mapping.StartMode = startModeOrDefault(update.StartMode)
			mapping.WatermarkMode = watermarkModeOrDefault(nil, mapping.StartMode)
			if syncEngine != nil {
				watermark, err := syncEngine.InitialWatermarkForStartMode(r.Context(), mapping.SonarrSeriesID, mapping.AFLSlug, mapping.StartMode)
				if err != nil {
					writeError(w, http.StatusBadGateway, "failed to compute the starting watermark: "+err.Error())
					return
				}
				mapping.SyncWatermark = watermark
			}
		}
		if update.WatermarkMode != nil {
			mapping.WatermarkMode = watermarkModeOrDefault(update.WatermarkMode, mapping.StartMode)
			if mapping.WatermarkMode == models.WatermarkModeAuto && syncEngine != nil {
				watermark, err := syncEngine.InitialWatermarkForSeries(r.Context(), mapping.SonarrSeriesID, mapping.AFLSlug)
				if err != nil {
					writeError(w, http.StatusBadGateway, "failed to compute the starting watermark: "+err.Error())
					return
				}
				if watermark > mapping.SyncWatermark {
					mapping.SyncWatermark = watermark
				}
			}
		}
		if update.SyncWatermark != nil {
			mapping.SyncWatermark = clampWatermark(update.SyncWatermark)
			mapping.WatermarkMode = models.WatermarkModeManual
		}

		// Mark the mapping user-modified so an in-flight sync skips its stale
		// write-backs (watermark, episodes, monitor changes) instead of
		// reverting this edit. This must stay BEFORE UpsertMapping: bumping
		// after the write would let a sync that already passed its supersede
		// check land its stale write-back in between and clobber the new
		// state. The cost is that a failed UpsertMapping leaves the epoch
		// bumped, so in-flight syncs skip that mapping once — harmless, the
		// next sync picks up its state.
		if syncEngine != nil {
			syncEngine.BumpMappingEpoch(id)
		}
		if err := db.UpsertMapping(r.Context(), mapping); err != nil {
			writeInternalError(w, err, "failed to update mapping")
			return
		}
		if update.NotificationsMuted != nil {
			if err := db.SetNotificationsMuted(r.Context(), mapping.ID, mapping.NotificationsMuted); err != nil {
				writeInternalError(w, err, "failed to update mapping")
				return
			}
		}
		if syncEngine != nil {
			hydratedMapping, err := syncEngine.HydrateMapping(r.Context(), mapping.ID)
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to hydrate mapping episodes: "+err.Error())
				return
			}
			if hydratedMapping != nil {
				mapping = hydratedMapping
			}
		}
		writeJSON(w, http.StatusOK, mapping)
	}
}

func handleDeleteMapping(db *database.DB, mgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		if err := db.DeleteMapping(r.Context(), id); err != nil {
			writeInternalError(w, err, "failed to delete mapping")
			return
		}
		// Clean up downloaded artwork files
		mgr.DeleteArtwork(id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
