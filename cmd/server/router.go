package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	"mizu/internal/sonarr"
	"mizu/internal/sync"
	"mizu/internal/web"
)

func setupRouter(cfg *config.Config, db *database.DB, syncEngine *sync.Engine, scheduler *schedulerController) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			status := "ok"
			if cfg.SonarrURL != "" {
				c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 30*time.Second)
				if _, err := c.TestConnection(r.Context()); err != nil {
					status = "degraded"
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"status": status, "version": version})
		})

		r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"version": version, "commit": commit, "build_date": buildDate})
		})

		r.Get("/stats", handleGetStats(db))
		r.Route("/mappings", func(r chi.Router) {
			r.Get("/", handleGetMappings(db))
			r.Post("/", handleCreateMapping(cfg, db, syncEngine))
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", handleGetMapping(db))
				r.Put("/", handleUpdateMapping(syncEngine, db))
				r.Delete("/", handleDeleteMapping(db))
				r.Get("/episodes", handleGetEpisodes(cfg, db, syncEngine))
				r.Get("/episodes/sonarr", handleGetSonarrEpisodes(cfg, db))
				r.Put("/episodes/{epId}/monitor", handleSetEpisodeMonitor(cfg, db))
				r.Post("/resync", handleResyncMapping(syncEngine))
			})
		})

		r.Post("/sync", handleTriggerSync(syncEngine))
		r.Get("/logs", handleGetLogs(db))
		r.Get("/settings", handleGetSettings(cfg, db))
		r.Get("/settings/sonarr-api-key", handleGetStoredSonarrAPIKey(cfg))
		r.Put("/settings", handleUpdateSettings(cfg, db, scheduler))
		r.Post("/settings/afl-cache/refresh", handleRefreshAFLCache(syncEngine, db))
		r.Get("/sonarr/series", handleGetSonarrSeries(cfg))
		r.Post("/sonarr/test", handleTestSonarr())
		r.Get("/afl/search", handleAFLSearch(syncEngine))
		r.Get("/afl/suggest", handleAFLSuggest(syncEngine))
	})

	r.NotFound(web.SPAHandler().ServeHTTP)
	return r
}

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

func handleGetMappings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
			writeError(w, http.StatusInternalServerError, err.Error())
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

func handleCreateMapping(cfg *config.Config, db *database.DB, syncEngine *sync.Engine) http.HandlerFunc {
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
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.SonarrSeriesID == 0 {
			writeError(w, http.StatusBadRequest, "sonarr_series_id is required")
			return
		}

		if req.AutoMatch && req.AFLSlug == "" {
			if syncEngine == nil {
				writeError(w, http.StatusInternalServerError, "auto-match unavailable")
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
			writeJSON(w, http.StatusCreated, storedMapping)
			return
		}

		if req.AFLSlug == "" {
			writeError(w, http.StatusBadRequest, "afl_slug is required for manual mapping")
			return
		}

		sonarrTitle := req.SonarrTitle
		if sonarrTitle == "" {
			c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
			sonarrSeries, err := c.GetSeries(r.Context(), req.SonarrSeriesID)
			if err == nil && sonarrSeries != nil {
				sonarrTitle = sonarrSeries.Title
			}
		}

		startMode := startModeOrDefault(req.StartMode)
		initialWatermark := 0
		if syncEngine != nil {
			watermark, err := syncEngine.InitialWatermarkForStartMode(r.Context(), req.SonarrSeriesID, startMode)
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to inspect Sonarr episodes: "+err.Error())
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

		mapping := models.ShowMapping{
			SonarrSeriesID:   req.SonarrSeriesID,
			SonarrTitle:      sonarrTitle,
			AFLSlug:          req.AFLSlug,
			AFLTitle:         req.AFLTitle,
			AutoMatched:      false,
			MatchConfidence:  1.0,
			FillerAction:     actionOrDefault(req.FillerAction, cfg.DefaultFillerAction),
			MixedAction:      actionOrDefault(req.MixedAction, cfg.DefaultMixedCanonAction),
			AnimeCanonAction: actionOrDefault(req.AnimeCanonAction, cfg.DefaultAnimeCanonAction),
			Enabled:          enabled,
			StartMode:        startMode,
			WatermarkMode:    watermarkMode,
			SyncWatermark:    initialWatermark,
		}
		if err := db.UpsertMapping(r.Context(), &mapping); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save mapping: "+err.Error())
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

		writeJSON(w, http.StatusCreated, storedMapping)
	}
}

func handleUpdateMapping(syncEngine *sync.Engine, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil || mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}

		var update struct {
			FillerAction     *string `json:"filler_action"`
			MixedAction      *string `json:"mixed_action"`
			AnimeCanonAction *string `json:"anime_canon_action"`
			Enabled          *bool   `json:"enabled"`
			AFLSlug          *string `json:"afl_slug"`
			StartMode        *string `json:"start_mode"`
			WatermarkMode    *string `json:"watermark_mode"`
			SyncWatermark    *int    `json:"sync_watermark"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
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
		if update.AFLSlug != nil {
			mapping.AFLSlug = *update.AFLSlug
		}
		if update.StartMode != nil {
			mapping.StartMode = startModeOrDefault(update.StartMode)
			mapping.WatermarkMode = watermarkModeOrDefault(nil, mapping.StartMode)
			if syncEngine != nil {
				watermark, err := syncEngine.InitialWatermarkForStartMode(r.Context(), mapping.SonarrSeriesID, mapping.StartMode)
				if err != nil {
					writeError(w, http.StatusBadGateway, "failed to inspect Sonarr episodes: "+err.Error())
					return
				}
				mapping.SyncWatermark = watermark
			}
		}
		if update.WatermarkMode != nil {
			mapping.WatermarkMode = watermarkModeOrDefault(update.WatermarkMode, mapping.StartMode)
			if mapping.WatermarkMode == models.WatermarkModeAuto && syncEngine != nil {
				watermark, err := syncEngine.InitialWatermarkForSeries(r.Context(), mapping.SonarrSeriesID)
				if err != nil {
					writeError(w, http.StatusBadGateway, "failed to inspect Sonarr episodes: "+err.Error())
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

		if err := db.UpsertMapping(r.Context(), mapping); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update mapping: "+err.Error())
			return
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

func handleDeleteMapping(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		if err := db.DeleteMapping(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete mapping: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func handleGetEpisodes(cfg *config.Config, db *database.DB, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil || mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}
		if syncEngine == nil {
			writeError(w, http.StatusInternalServerError, "sync engine unavailable")
			return
		}

		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		sonarrEpisodes, err := c.GetEpisodes(r.Context(), mapping.SonarrSeriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch episodes from Sonarr: "+err.Error())
			return
		}

		_, aflEpisodes, err := syncEngine.ScraperClient().GetShowEpisodes(r.Context(), mapping.AFLSlug)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch AFL episodes: "+err.Error())
			return
		}

		aflByNumber, aflPreferred := sync.BuildAFLEpisodeLookup(aflEpisodes)
		enriched := make([]models.EpisodeInfo, 0, len(sonarrEpisodes))
		for _, ep := range sonarrEpisodes {
			aflNum, aflEpisode, found := sync.SelectAFLEpisodeForSonarr(ep, aflByNumber, aflPreferred)

			typeValue := models.EpisodeUnknown
			if found {
				typeValue = aflEpisode.Type
			} else if ep.SeasonNumber == 0 || ep.SeasonNumber == -1 {
				typeValue = models.EpisodeSpecial
			}

			episodeNum := aflNum
			aflEpisodeNum := aflNum
			if ep.SeasonNumber == 0 || ep.SeasonNumber == -1 {
				episodeNum = ep.EpisodeNumber
			} else if episodeNum <= 0 {
				if ep.AbsoluteEpisodeNumber != nil && *ep.AbsoluteEpisodeNumber > 0 {
					episodeNum = *ep.AbsoluteEpisodeNumber
				} else {
					episodeNum = ep.EpisodeNumber
				}
			}

			enriched = append(enriched, models.EpisodeInfo{
				MappingID:             id,
				EpisodeNum:            episodeNum,
				AFLEpisodeNumber:      aflEpisodeNum,
				Title:                 ep.Title,
				Type:                  typeValue,
				SonarrEpisodeID:       ep.ID,
				SeasonNumber:          ep.SeasonNumber,
				SonarrEpisodeNumber:   ep.EpisodeNumber,
				AbsoluteEpisodeNumber: ep.AbsoluteEpisodeNumber,
				Monitored:             ep.Monitored,
			})
		}

		writeJSON(w, http.StatusOK, enriched)
	}
}
func handleGetSonarrEpisodes(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil || mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}
		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		episodes, err := c.GetEpisodes(r.Context(), mapping.SonarrSeriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch episodes from Sonarr: "+err.Error())
			return
		}

		seasons := make(map[int][]models.SonarrEpisode)
		for _, ep := range episodes {
			seasons[ep.SeasonNumber] = append(seasons[ep.SeasonNumber], ep)
		}
		writeJSON(w, http.StatusOK, seasons)
	}
}

func handleSetEpisodeMonitor(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		epID, err := strconv.Atoi(chi.URLParam(r, "epId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid episode ID")
			return
		}
		var req struct {
			Monitored bool `json:"monitored"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		if err := c.SetEpisodeMonitored(r.Context(), epID, req.Monitored); err != nil {
			writeError(w, http.StatusBadGateway, "failed to update episode in Sonarr: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"monitored": req.Monitored})
	}
}

func handleResyncMapping(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		go func() {
			if err := engine.RunResync(context.Background(), id); err != nil {
				log.Error().Err(err).Int64("mapping_id", id).Msg("resync failed")
			}
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "resync triggered"})
	}
}

func handleTriggerSync(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		go func() {
			if err := engine.RunSync(context.Background(), "manual"); err != nil {
				log.Error().Err(err).Msg("manual sync failed")
			}
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync triggered"})
	}
}

func handleGetLogs(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		offset := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
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

func handleGetSettings(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maskedKey := ""
		if cfg.SonarrAPIKey != "" && len(cfg.SonarrAPIKey) >= 8 {
			maskedKey = cfg.SonarrAPIKey[:4] + "..." + cfg.SonarrAPIKey[len(cfg.SonarrAPIKey)-4:]
		}
		cacheCount, cacheRefreshedAt, err := db.GetAFLShowCacheStatus(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sonarr_url":                  cfg.SonarrURL,
			"sonarr_api_key_masked":       maskedKey,
			"sonarr_api_key_set":          cfg.SonarrAPIKey != "",
			"default_filler_action":       string(cfg.DefaultFillerAction),
			"default_mixed_canon_action":  string(cfg.DefaultMixedCanonAction),
			"default_anime_canon_action":  string(cfg.DefaultAnimeCanonAction),
			"sync_interval":               scanIntervalToValue(cfg.ScanInterval),
			"enable_tagging":              cfg.EnableTagging,
			"filler_tag_name":             cfg.FillerTagName,
			"afl_show_cache_refreshed_at": cacheRefreshedAt,
			"afl_show_cache_count":        cacheCount,
		})
	}
}

func handleGetStoredSonarrAPIKey(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sonarr_api_key":     cfg.SonarrAPIKey,
			"sonarr_api_key_set": cfg.SonarrAPIKey != "",
		})
	}
}

func handleUpdateSettings(cfg *config.Config, db *database.DB, scheduler *schedulerController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var update struct {
			SonarrURL               *string `json:"sonarr_url"`
			SonarrAPIKey            *string `json:"sonarr_api_key"`
			DefaultFillerAction     *string `json:"default_filler_action"`
			DefaultMixedCanonAction *string `json:"default_mixed_canon_action"`
			DefaultAnimeCanonAction *string `json:"default_anime_canon_action"`
			SyncInterval            *string `json:"sync_interval"`
			EnableTagging           *bool   `json:"enable_tagging"`
			FillerTagName           *string `json:"filler_tag_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if update.SonarrURL != nil {
			cfg.SonarrURL = *update.SonarrURL
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
			normalized, d, err := normalizeScanInterval(*update.SyncInterval)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid sync interval")
				return
			}
			cfg.ScanInterval = d
			scheduler.UpdateInterval(d)
			update.SyncInterval = &normalized
		}
		if update.EnableTagging != nil {
			cfg.EnableTagging = *update.EnableTagging
		}
		if update.FillerTagName != nil && *update.FillerTagName != "" {
			cfg.FillerTagName = *update.FillerTagName
		}

		for k, v := range map[string]string{
			"sonarr_url":                 cfg.SonarrURL,
			"sonarr_api_key":             cfg.SonarrAPIKey,
			"default_filler_action":      string(cfg.DefaultFillerAction),
			"default_mixed_canon_action": string(cfg.DefaultMixedCanonAction),
			"default_anime_canon_action": string(cfg.DefaultAnimeCanonAction),
			"sync_interval":              scanIntervalToValue(cfg.ScanInterval),
			"enable_tagging":             strconv.FormatBool(cfg.EnableTagging),
			"filler_tag_name":            cfg.FillerTagName,
		} {
			if err := db.SetSetting(r.Context(), k, v); err != nil {
				log.Warn().Err(err).Str("key", k).Msg("failed to persist setting")
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "settings updated"})
	}
}
func handleRefreshAFLCache(engine *sync.Engine, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shows, err := engine.ScraperClient().RefreshAllShows(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to refresh AFL show list: "+err.Error())
			return
		}
		count, refreshedAt, err := db.GetAFLShowCacheStatus(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if count == 0 {
			count = len(shows)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "refreshed", "count": count, "refreshed_at": refreshedAt,
		})
	}
}

func handleTestSonarr() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL    string `json:"url"`
			APIKey string `json:"api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.URL == "" || req.APIKey == "" {
			writeError(w, http.StatusBadRequest, "url and api_key are required")
			return
		}
		c := sonarr.New(req.URL, req.APIKey, 10*time.Second)
		info, err := c.TestConnection(r.Context())
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "version": info})
	}
}

func handleAFLSearch(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			writeJSON(w, http.StatusOK, []models.AFLShow{})
			return
		}

		results, err := engine.ScraperClient().SearchShows(r.Context(), query)
		if err != nil {
			writeError(w, http.StatusBadGateway, "AFL search failed: "+err.Error())
			return
		}

		seriesIDParam := r.URL.Query().Get("series_id")
		if seriesIDParam == "" {
			writeJSON(w, http.StatusOK, results)
			return
		}

		seriesID, err := strconv.Atoi(seriesIDParam)
		if err != nil || seriesID == 0 {
			writeError(w, http.StatusBadRequest, "invalid series_id parameter")
			return
		}

		series, err := engine.SonarrClient().GetSeries(r.Context(), seriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load Sonarr series: "+err.Error())
			return
		}

		matches := engine.MatcherClient().FindMatches(r.Context(), *series, results)
		writeJSON(w, http.StatusOK, matches)
	}
}

func handleAFLSuggest(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seriesID, err := strconv.Atoi(r.URL.Query().Get("series_id"))
		if err != nil || seriesID == 0 {
			writeError(w, http.StatusBadRequest, "series_id parameter is required")
			return
		}
		_, matches, err := engine.SuggestMatches(r.Context(), seriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "AFL suggest failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, matches)
	}
}

func handleGetSonarrSeries(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		series, err := c.GetAnimeSeries(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr series: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, series)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
