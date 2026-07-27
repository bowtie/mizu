package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	stdsync "sync"
	"time"

	"mizu/internal/anilist"
	"mizu/internal/config"
	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
	"mizu/internal/sonarr"
	"mizu/internal/sync"
	"mizu/internal/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
)

// episodeCache provides a short-lived in-memory cache for enriched episode
// data so that navigating to an anime page doesn't always hit Sonarr + AFL.
type episodeCache struct {
	mu      stdsync.RWMutex
	entries map[int64]episodeCacheEntry
	ttl     time.Duration
}

type episodeCacheEntry struct {
	data      []models.EpisodeInfo
	expiresAt time.Time
}

func newEpisodeCache(ttl time.Duration) *episodeCache {
	return &episodeCache{entries: make(map[int64]episodeCacheEntry), ttl: ttl}
}

func (c *episodeCache) get(mappingID int64) ([]models.EpisodeInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[mappingID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

func (c *episodeCache) set(mappingID int64, data []models.EpisodeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[mappingID] = episodeCacheEntry{data: data, expiresAt: time.Now().Add(c.ttl)}
}

func (c *episodeCache) invalidate(mappingID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, mappingID)
}

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

	epCache := newEpisodeCache(2 * time.Minute)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			status := "ok"
			if cfg.SonarrURL != "" {
				c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 30*time.Second)
				if _, err := c.TestConnection(r.Context()); err != nil {
					status = "degraded"
				}
			}
			payload := map[string]interface{}{"status": status, "version": version}
			if latest, err := db.GetLatestSyncLog(r.Context()); err == nil {
				payload["last_sync_at"] = latest.CompletedAt
				interval := scheduler.Interval()
				if interval > 0 {
					next := latest.CompletedAt.Add(interval)
					if next.Before(time.Now()) {
						next = time.Now().Add(interval)
					}
					payload["next_sync_at"] = next
				}
			}
			writeJSON(w, http.StatusOK, payload)
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
				r.Get("/artwork", handleGetMappingArtwork(cfg, db))
				r.Get("/episodes", handleGetEpisodes(cfg, db, syncEngine, epCache))
				r.Get("/episodes/sonarr", handleGetSonarrEpisodes(cfg, db))
				r.Put("/episodes/{epId}/monitor", handleSetEpisodeMonitor(cfg, db, epCache))
				r.Post("/resync", handleResyncMapping(syncEngine))
			})
		})

		r.Post("/sync", handleTriggerSync(syncEngine))
		r.Get("/logs", handleGetLogs(db))
		r.Get("/settings", handleGetSettings(cfg, db))
		r.Get("/settings/sonarr-api-key", handleGetStoredSonarrAPIKey(cfg))
		r.Put("/settings", handleUpdateSettings(cfg, db, scheduler))
		r.Post("/settings/afl-cache/refresh", handleRefreshAFLCache(syncEngine, db))
		r.Get("/sonarr/series", handleGetSonarrSeries(cfg, db, syncEngine))
		r.Get("/sonarr/series/{id}", handleGetSonarrSeriesByID(cfg, db))
		r.Post("/sonarr/series", handleAddSonarrSeries(cfg))
		r.Get("/sonarr/lookup", handleLookupSonarrSeries(cfg))
		r.Get("/sonarr/add-options", handleGetSonarrAddOptions(cfg))
		r.Post("/sonarr/manual-match-pending", handleMarkManualMatchPending(db))
		r.Post("/sonarr/test", handleTestSonarr())
		r.Get("/afl/search", handleAFLSearch(syncEngine, db))
		r.Get("/afl/suggest", handleAFLSuggest(syncEngine, db))
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

const manualMatchPendingSeriesIDsKey = "manual_match_pending_series_ids"

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
			if err := clearManualMatchPendingSeriesID(r.Context(), db, req.SonarrSeriesID); err != nil {
				log.Warn().Err(err).Int("sonarr_series_id", req.SonarrSeriesID).Msg("failed to clear manual-match pending series")
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

		if err := clearManualMatchPendingSeriesID(r.Context(), db, req.SonarrSeriesID); err != nil {
			log.Warn().Err(err).Int("sonarr_series_id", req.SonarrSeriesID).Msg("failed to clear manual-match pending series")
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

func handleGetEpisodes(cfg *config.Config, db *database.DB, syncEngine *sync.Engine, epCache *episodeCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		// Serve from in-memory cache if available
		if cached, ok := epCache.get(id); ok {
			writeJSON(w, http.StatusOK, cached)
			return
		}

		// Serve from database (populated by syncs) — instant, no external calls
		dbEpisodes, err := db.GetEpisodes(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load episodes: "+err.Error())
			return
		}

		if len(dbEpisodes) > 0 && hasSonarrEpisodeIDs(dbEpisodes) {
			epCache.set(id, dbEpisodes)
			writeJSON(w, http.StatusOK, dbEpisodes)
			return
		}

		// DB is empty — fetch live from Sonarr + AFL as a fallback
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil || mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}
		if syncEngine == nil {
			writeError(w, http.StatusInternalServerError, "sync engine unavailable")
			return
		}

		enriched, herr := fetchEpisodesLive(cfg, syncEngine, id, mapping.SonarrSeriesID, mapping.AFLSlug)
		if herr != nil {
			writeError(w, herr.code, herr.message)
			return
		}

		// Persist to DB so future loads are instant
		_ = db.ReplaceEpisodesForMapping(r.Context(), id, enriched)

		epCache.set(id, enriched)
		writeJSON(w, http.StatusOK, enriched)
	}
}

func hasSonarrEpisodeIDs(episodes []models.EpisodeInfo) bool {
	for _, ep := range episodes {
		if ep.SonarrEpisodeID != 0 {
			return true
		}
	}
	return false
}

type httpError struct {
	code    int
	message string
}

func (e *httpError) Error() string { return e.message }

func fetchEpisodesLive(cfg *config.Config, syncEngine *sync.Engine, mappingID int64, sonarrSeriesID int, aflSlug string) ([]models.EpisodeInfo, *httpError) {
	c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
	sonarrEpisodes, err := c.GetEpisodes(context.Background(), sonarrSeriesID)
	if err != nil {
		return nil, &httpError{http.StatusBadGateway, "failed to fetch episodes from Sonarr: " + err.Error()}
	}

	_, aflEpisodes, err := syncEngine.ScraperClient().GetShowEpisodes(context.Background(), aflSlug)
	if err != nil {
		return nil, &httpError{http.StatusBadGateway, "failed to fetch AFL episodes: " + err.Error()}
	}

	aflByNumber, aflPreferred := sync.BuildAFLEpisodeLookup(aflEpisodes)
	coverageMax := sync.AFLCoverageMaxEpisodeNumber(aflEpisodes)
	enriched := make([]models.EpisodeInfo, 0, len(sonarrEpisodes))
	for _, ep := range sonarrEpisodes {
		aflNum, aflEpisode, found := sync.SelectAFLEpisodeForSonarr(ep, aflByNumber, aflPreferred)
		if !sync.IsWithinAFLCoverage(ep, aflNum, found, coverageMax) {
			continue
		}

		typeValue := models.EpisodeUnknown
		if found {
			typeValue = aflEpisode.Type
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
			MappingID:             mappingID,
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

	return enriched, nil
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

func handleSetEpisodeMonitor(cfg *config.Config, db *database.DB, epCache *episodeCache) http.HandlerFunc {
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
		// Invalidate cache for this mapping so the next fetch reflects the change
		if id, idErr := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64); idErr == nil {
			epCache.invalidate(id)
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
			"sonarr_url":                    cfg.SonarrURL,
			"sonarr_api_key_masked":         maskedKey,
			"sonarr_api_key_set":            cfg.SonarrAPIKey != "",
			"default_filler_action":         string(cfg.DefaultFillerAction),
			"default_mixed_canon_action":    string(cfg.DefaultMixedCanonAction),
			"default_anime_canon_action":    string(cfg.DefaultAnimeCanonAction),
			"sync_interval":                 scanIntervalToValue(cfg.ScanInterval),
			"enable_anilist_artwork":        cfg.EnableAniListArtwork,
			"sonarr_add_root_folder":        cfg.SonarrAddRootFolder,
			"sonarr_add_quality_profile_id": cfg.SonarrAddQualityProfileID,
			"sonarr_add_monitor_mode":       cfg.SonarrAddMonitorMode,
			"sonarr_add_season_folder":      cfg.SonarrAddSeasonFolder,
			"afl_show_cache_refreshed_at":   cacheRefreshedAt,
			"afl_show_cache_count":          cacheCount,
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
			SonarrURL                 *string `json:"sonarr_url"`
			SonarrAPIKey              *string `json:"sonarr_api_key"`
			DefaultFillerAction       *string `json:"default_filler_action"`
			DefaultMixedCanonAction   *string `json:"default_mixed_canon_action"`
			DefaultAnimeCanonAction   *string `json:"default_anime_canon_action"`
			SyncInterval              *string `json:"sync_interval"`
			EnableAniListArtwork      *bool   `json:"enable_anilist_artwork"`
			SonarrAddRootFolder       *string `json:"sonarr_add_root_folder"`
			SonarrAddQualityProfileID *int    `json:"sonarr_add_quality_profile_id"`
			SonarrAddMonitorMode      *string `json:"sonarr_add_monitor_mode"`
			SonarrAddSeasonFolder     *bool   `json:"sonarr_add_season_folder"`
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
		if update.EnableAniListArtwork != nil {
			cfg.EnableAniListArtwork = *update.EnableAniListArtwork
		}
		if update.SonarrAddRootFolder != nil {
			cfg.SonarrAddRootFolder = *update.SonarrAddRootFolder
		}
		if update.SonarrAddQualityProfileID != nil {
			cfg.SonarrAddQualityProfileID = *update.SonarrAddQualityProfileID
		}
		if update.SonarrAddMonitorMode != nil {
			if !configIsValidSonarrMonitorMode(*update.SonarrAddMonitorMode) {
				writeError(w, http.StatusBadRequest, "invalid Sonarr monitor mode")
				return
			}
			cfg.SonarrAddMonitorMode = *update.SonarrAddMonitorMode
		}
		if update.SonarrAddSeasonFolder != nil {
			cfg.SonarrAddSeasonFolder = *update.SonarrAddSeasonFolder
		}

		for k, v := range map[string]string{
			"sonarr_url":                    cfg.SonarrURL,
			"sonarr_api_key":                cfg.SonarrAPIKey,
			"default_filler_action":         string(cfg.DefaultFillerAction),
			"default_mixed_canon_action":    string(cfg.DefaultMixedCanonAction),
			"default_anime_canon_action":    string(cfg.DefaultAnimeCanonAction),
			"sync_interval":                 scanIntervalToValue(cfg.ScanInterval),
			"enable_anilist_artwork":        strconv.FormatBool(cfg.EnableAniListArtwork),
			"sonarr_add_root_folder":        cfg.SonarrAddRootFolder,
			"sonarr_add_quality_profile_id": strconv.Itoa(cfg.SonarrAddQualityProfileID),
			"sonarr_add_monitor_mode":       cfg.SonarrAddMonitorMode,
			"sonarr_add_season_folder":      strconv.FormatBool(cfg.SonarrAddSeasonFolder),
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

func handleAFLSearch(engine *sync.Engine, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		cachedShows, err := loadCachedAFLShows(r.Context(), db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load cached AFL shows: "+err.Error())
			return
		}

		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mappings: "+err.Error())
			return
		}

		results := excludeMappedAFLShows(filterCachedAFLShows(query, cachedShows), mappings, 0)

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

		matches := excludeMappedMatchResults(matchCachedAFLShows(engine.MatcherClient(), *series, results), mappings, seriesID)
		if len(matches) == 0 {
			writeJSON(w, http.StatusOK, excludeMappedAFLShows(results, mappings, seriesID))
			return
		}
		writeJSON(w, http.StatusOK, matches)
	}
}

func handleAFLSuggest(engine *sync.Engine, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seriesID, err := strconv.Atoi(r.URL.Query().Get("series_id"))
		if err != nil || seriesID == 0 {
			writeError(w, http.StatusBadRequest, "series_id parameter is required")
			return
		}

		cachedShows, err := loadCachedAFLShows(r.Context(), db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load cached AFL shows: "+err.Error())
			return
		}

		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mappings: "+err.Error())
			return
		}

		series, err := engine.SonarrClient().GetSeries(r.Context(), seriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load Sonarr series: "+err.Error())
			return
		}

		matches := excludeMappedMatchResults(matchCachedAFLShows(engine.MatcherClient(), *series, cachedShows), mappings, seriesID)
		writeJSON(w, http.StatusOK, matches)
	}
}

func loadCachedAFLShows(ctx context.Context, db *database.DB) ([]models.AFLShow, error) {
	shows, _, err := db.GetCachedAFLShows(ctx)
	if err != nil {
		return nil, err
	}
	return shows, nil
}

func matchCachedAFLShows(m *matcherpkg.Matcher, series models.SonarrSeries, shows []models.AFLShow) []matcherpkg.MatchResult {
	return m.FindMatches(context.Background(), series, shows)
}

func filterCachedAFLShows(query string, shows []models.AFLShow) []models.AFLShow {
	normalizer := matcherpkg.New(0.82)
	normalizedQuery := normalizer.Normalize(query)
	if normalizedQuery == "" {
		results := append([]models.AFLShow(nil), shows...)
		sort.SliceStable(results, func(i, j int) bool {
			return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
		})
		return results
	}

	type scoredShow struct {
		show  models.AFLShow
		score int
	}

	queryTokens := strings.Fields(normalizedQuery)
	scored := make([]scoredShow, 0, len(shows))

	for _, show := range shows {
		normalizedTitle := normalizer.Normalize(show.Title)
		if normalizedTitle == "" {
			continue
		}

		score := 0
		switch {
		case normalizedTitle == normalizedQuery:
			score = 1000
		case strings.HasPrefix(normalizedTitle, normalizedQuery):
			score = 920
		case strings.Contains(normalizedTitle, normalizedQuery):
			score = 860
		default:
			shared := sharedCachedSearchTokens(queryTokens, normalizedTitle)
			if shared == 0 {
				continue
			}
			score = 700 + shared*30 + int(matcherpkg.JaroWinkler(normalizedQuery, normalizedTitle)*100)
		}

		scored = append(scored, scoredShow{show: show, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return strings.ToLower(scored[i].show.Title) < strings.ToLower(scored[j].show.Title)
		}
		return scored[i].score > scored[j].score
	})

	results := make([]models.AFLShow, 0, len(scored))
	for _, entry := range scored {
		results = append(results, entry.show)
	}
	return results
}

func sharedCachedSearchTokens(queryTokens []string, normalizedTitle string) int {
	if len(queryTokens) == 0 || normalizedTitle == "" {
		return 0
	}

	seen := make(map[string]struct{}, len(queryTokens))
	for _, token := range queryTokens {
		if len(token) < 2 {
			continue
		}
		seen[token] = struct{}{}
	}

	shared := 0
	used := make(map[string]struct{}, len(queryTokens))
	for _, token := range strings.Fields(normalizedTitle) {
		if len(token) < 2 {
			continue
		}
		if _, ok := used[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			shared++
			used[token] = struct{}{}
		}
	}

	return shared
}

func handleGetSonarrAddOptions(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		rootFolders, err := c.GetRootFolders(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr root folders: "+err.Error())
			return
		}
		qualityProfiles, err := c.GetQualityProfiles(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr quality profiles: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"root_folders":     rootFolders,
			"quality_profiles": qualityProfiles,
		})
	}
}

func handleLookupSonarrSeries(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusOK, []models.SonarrSeries{})
			return
		}

		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		results, err := c.LookupSeries(r.Context(), query)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to lookup Sonarr series: "+err.Error())
			return
		}

		anime := make([]models.SonarrSeries, 0, len(results))
		for _, item := range results {
			if item.SeriesType == "" || item.SeriesType == "anime" || item.SeriesType == "standard" {
				anime = append(anime, item)
			}
		}
		writeJSON(w, http.StatusOK, anime)
	}
}

func handleAddSonarrSeries(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Series           models.SonarrSeries `json:"series"`
			RootFolderPath   *string             `json:"root_folder_path"`
			QualityProfileID *int                `json:"quality_profile_id"`
			MonitorMode      *string             `json:"monitor_mode"`
			SeasonFolder     *bool               `json:"season_folder"`
			SearchOnAdd      *bool               `json:"search_on_add"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.Series.Title == "" {
			writeError(w, http.StatusBadRequest, "series is required")
			return
		}

		rootFolder := cfg.SonarrAddRootFolder
		if req.RootFolderPath != nil {
			rootFolder = *req.RootFolderPath
		}
		qualityProfileID := cfg.SonarrAddQualityProfileID
		if req.QualityProfileID != nil {
			qualityProfileID = *req.QualityProfileID
		}
		monitorMode := "none"
		if req.MonitorMode != nil {
			monitorMode = *req.MonitorMode
		}
		seasonFolder := true
		if req.SeasonFolder != nil {
			seasonFolder = *req.SeasonFolder
		}
		searchOnAdd := false
		if req.SearchOnAdd != nil {
			searchOnAdd = *req.SearchOnAdd
		}

		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		if rootFolder == "" {
			animeSeries, err := c.GetAnimeSeries(r.Context())
			if err != nil {
				log.Warn().Err(err).Msg("failed to load anime library while resolving Sonarr root folder")
			}
			rootFolders, err := c.GetRootFolders(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to fetch Sonarr root folders: "+err.Error())
				return
			}
			rootFolder = preferredSonarrRootFolder(animeSeries, rootFolders)
		}

		if rootFolder == "" || qualityProfileID <= 0 {
			writeError(w, http.StatusBadRequest, "quality profile is required before adding to Sonarr")
			return
		}
		if !configIsValidSonarrMonitorMode(monitorMode) {
			writeError(w, http.StatusBadRequest, "invalid Sonarr monitor mode")
			return
		}

		created, err := c.AddSeries(r.Context(), req.Series, sonarr.AddSeriesOptions{
			RootFolderPath:   rootFolder,
			QualityProfileID: qualityProfileID,
			SeasonFolder:     seasonFolder,
			MonitorMode:      monitorMode,
			SearchOnAdd:      searchOnAdd,
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to add Sonarr series: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

func preferredSonarrRootFolder(series []models.SonarrSeries, rootFolders []models.SonarrRootFolder) string {
	counts := make(map[string]int)
	bestPath := ""
	bestCount := 0

	for _, item := range series {
		path := strings.TrimSpace(item.RootFolderPath)
		if path == "" {
			continue
		}
		counts[path]++
		if counts[path] > bestCount {
			bestCount = counts[path]
			bestPath = path
		}
	}

	if bestPath != "" {
		return bestPath
	}
	if len(rootFolders) > 0 {
		return strings.TrimSpace(rootFolders[0].Path)
	}
	return ""
}

func handleMarkManualMatchPending(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SonarrSeriesID int `json:"sonarr_series_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.SonarrSeriesID == 0 {
			writeError(w, http.StatusBadRequest, "sonarr_series_id is required")
			return
		}
		if err := markManualMatchPendingSeriesID(r.Context(), db, req.SonarrSeriesID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to persist pending manual match: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "pending", "sonarr_series_id": req.SonarrSeriesID})
	}
}

func handleGetSonarrSeriesByID(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid series ID")
			return
		}

		// Serve from cache if fresh (< 24h)
		if cached, ok := db.GetCachedSonarrSeries(r.Context(), id, 24*time.Hour); ok {
			writeJSON(w, http.StatusOK, cached)
			return
		}

		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		series, err := c.GetSeries(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr series: "+err.Error())
			return
		}
		if series == nil {
			writeError(w, http.StatusNotFound, "series not found")
			return
		}

		// Cache the result
		_ = db.SetCachedSonarrSeries(r.Context(), id, series)

		writeJSON(w, http.StatusOK, series)
	}
}

func handleGetSonarrSeries(cfg *config.Config, db *database.DB, engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
		series, err := c.GetAnimeSeries(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr series: "+err.Error())
			return
		}

		if r.URL.Query().Get("matchable_only") == "true" {
			mappings, err := db.GetAllMappings(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load mappings: "+err.Error())
				return
			}

			cachedShows, err := loadCachedAFLShows(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load cached AFL shows: "+err.Error())
				return
			}

			pendingManualMatchIDs, err := loadManualMatchPendingSeriesIDs(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load pending manual matches: "+err.Error())
				return
			}

			series = filterSeriesForAddModal(r.Context(), series, mappings, pendingManualMatchIDs, engine.MatcherClient(), cachedShows)
		}

		writeJSON(w, http.StatusOK, series)
	}
}

func filterSeriesForAddModal(ctx context.Context, series []models.SonarrSeries, mappings []models.ShowMapping, pendingManualMatchIDs map[int]struct{}, matcher *matcherpkg.Matcher, cachedShows []models.AFLShow) []models.SonarrSeries {
	mappedIDs := make(map[int]struct{}, len(mappings))
	for _, mapping := range mappings {
		mappedIDs[mapping.SonarrSeriesID] = struct{}{}
	}

	filtered := make([]models.SonarrSeries, 0, len(series))
	for _, item := range series {
		if _, ok := mappedIDs[item.ID]; ok {
			filtered = append(filtered, item)
			continue
		}
		if _, ok := pendingManualMatchIDs[item.ID]; ok {
			item.ManualMatchPending = true
			filtered = append(filtered, item)
			continue
		}

		match := matcher.BestMatch(ctx, item, cachedShows)
		if match != nil && !isMappedAFLSlugUnavailable(match.AFLShow.Slug, mappings, item.ID) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func excludeMappedMatchResults(matches []matcherpkg.MatchResult, mappings []models.ShowMapping, currentSeriesID int) []matcherpkg.MatchResult {
	filtered := make([]matcherpkg.MatchResult, 0, len(matches))
	for _, match := range matches {
		if isMappedAFLSlugUnavailable(match.AFLShow.Slug, mappings, currentSeriesID) {
			continue
		}
		filtered = append(filtered, match)
	}
	return filtered
}

func excludeMappedAFLShows(shows []models.AFLShow, mappings []models.ShowMapping, currentSeriesID int) []models.AFLShow {
	filtered := make([]models.AFLShow, 0, len(shows))
	for _, show := range shows {
		if isMappedAFLSlugUnavailable(show.Slug, mappings, currentSeriesID) {
			continue
		}
		filtered = append(filtered, show)
	}
	return filtered
}

func isMappedAFLSlugUnavailable(slug string, mappings []models.ShowMapping, currentSeriesID int) bool {
	if slug == "" {
		return false
	}

	for _, mapping := range mappings {
		if mapping.AFLSlug == slug && mapping.SonarrSeriesID != currentSeriesID {
			return true
		}
	}

	return false
}

func loadManualMatchPendingSeriesIDs(ctx context.Context, db *database.DB) (map[int]struct{}, error) {
	value, err := db.GetSetting(ctx, manualMatchPendingSeriesIDsKey)
	if err != nil {
		return nil, err
	}

	ids := make(map[int]struct{})
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, parseErr := strconv.Atoi(part)
		if parseErr != nil || id <= 0 {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

func storeManualMatchPendingSeriesIDs(ctx context.Context, db *database.DB, ids map[int]struct{}) error {
	values := make([]int, 0, len(ids))
	for id := range ids {
		values = append(values, id)
	}
	sort.Ints(values)

	parts := make([]string, 0, len(values))
	for _, id := range values {
		parts = append(parts, strconv.Itoa(id))
	}
	return db.SetSetting(ctx, manualMatchPendingSeriesIDsKey, strings.Join(parts, ","))
}

func markManualMatchPendingSeriesID(ctx context.Context, db *database.DB, seriesID int) error {
	ids, err := loadManualMatchPendingSeriesIDs(ctx, db)
	if err != nil {
		return err
	}
	ids[seriesID] = struct{}{}
	return storeManualMatchPendingSeriesIDs(ctx, db, ids)
}

func clearManualMatchPendingSeriesID(ctx context.Context, db *database.DB, seriesID int) error {
	ids, err := loadManualMatchPendingSeriesIDs(ctx, db)
	if err != nil {
		return err
	}
	delete(ids, seriesID)
	return storeManualMatchPendingSeriesIDs(ctx, db, ids)
}

func configIsValidSonarrMonitorMode(value string) bool {
	switch value {
	case "", "all", "future", "missing", "existing", "pilot", "firstSeason", "latestSeason", "none":
		return true
	default:
		return false
	}
}

func handleGetMappingArtwork(cfg *config.Config, db *database.DB) http.HandlerFunc {
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
		if !cfg.EnableAniListArtwork {
			writeJSON(w, http.StatusOK, map[string]interface{}{})
			return
		}

		series := models.SonarrSeries{
			ID:    mapping.SonarrSeriesID,
			Title: mapping.SonarrTitle,
		}

		if cfg.SonarrURL != "" {
			sonarrClient := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 15*time.Second)
			if fetched, fetchErr := sonarrClient.GetSeries(r.Context(), mapping.SonarrSeriesID); fetchErr == nil && fetched != nil {
				series = *fetched
			}
		}

		artwork, err := anilist.New(cfg.RequestTimeout).SearchArtwork(r.Context(), series)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch AniList artwork: "+err.Error())
			return
		}
		if artwork == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{})
			return
		}

		_ = db.UpdateAnilistArtwork(r.Context(), id, artwork.PosterImage, artwork.BannerImage)

		writeJSON(w, http.StatusOK, artwork)
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
