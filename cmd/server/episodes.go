package main

import (
	"context"
	"encoding/json"
	"mizu/internal/database"
	"mizu/internal/models"
	"mizu/internal/sync"
	"net/http"
	"strconv"
	stdsync "sync"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

func handleGetEpisodes(db *database.DB, syncEngine *sync.Engine, epCache *episodeCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		// Compute a cheap version tag for ETag-based caching.
		version, verr := db.EpisodeVersion(r.Context(), id)
		if verr == nil && version != "" {
			etag := `"` + version + `"`
			w.Header().Set("ETag", etag)
			// stale-while-revalidate lets the browser serve the cached
			// response instantly on repeat visits while revalidating
			// in the background. Combined with ETag, this makes repeat
			// visits to the same anime page effectively zero-latency.
			w.Header().Set("Cache-Control", "max-age=0, stale-while-revalidate=86400")
			if match := r.Header.Get("If-None-Match"); match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
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

		enriched, herr := fetchEpisodesLive(r.Context(), syncEngine, id, mapping.SonarrSeriesID, mapping.AFLSlug)
		if herr != nil {
			writeError(w, herr.code, herr.message)
			return
		}

		// Persist to DB so future loads are instant
		if err := db.ReplaceEpisodesForMapping(r.Context(), id, enriched); err != nil {
			log.Warn().Err(err).Int64("mapping_id", id).Msg("failed to persist live-fetched episodes")
		}

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

func fetchEpisodesLive(ctx context.Context, syncEngine *sync.Engine, mappingID int64, sonarrSeriesID int, aflSlug string) ([]models.EpisodeInfo, *httpError) {
	sonarrEpisodes, err := syncEngine.SonarrClient().GetEpisodes(ctx, sonarrSeriesID)
	if err != nil {
		return nil, &httpError{http.StatusBadGateway, "failed to fetch episodes from Sonarr: " + err.Error()}
	}

	_, aflEpisodes, err := syncEngine.ScraperClient().GetShowEpisodes(ctx, aflSlug)
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

		airDate := ep.AirDateUTC
		airDateOnly := models.DateOnly(airDate)
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
			AirDate:               &airDateOnly,
			Monitored:             ep.Monitored,
		})
	}

	return enriched, nil
}

func handleGetSonarrEpisodes(db *database.DB, syncEngine *sync.Engine) http.HandlerFunc {
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
		episodes, err := syncEngine.SonarrClient().GetEpisodes(r.Context(), mapping.SonarrSeriesID)
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

func handleSetEpisodeMonitor(db *database.DB, syncEngine *sync.Engine, epCache *episodeCache) http.HandlerFunc {
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
		if syncEngine == nil {
			writeError(w, http.StatusInternalServerError, "sync engine unavailable")
			return
		}
		id, idErr := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		// Mark the mapping user-modified before the Sonarr write so an
		// in-flight sync skips its stale write-backs instead of reverting
		// this toggle.
		if idErr == nil {
			syncEngine.BumpMappingEpoch(id)
		}
		if err := syncEngine.SonarrClient().SetEpisodeMonitored(r.Context(), epID, req.Monitored); err != nil {
			writeError(w, http.StatusBadGateway, "failed to update episode in Sonarr: "+err.Error())
			return
		}
		// Update the local DB so the change persists beyond cache invalidation
		if idErr == nil {
			_ = db.UpdateEpisodeMonitor(r.Context(), id, int64(epID), req.Monitored)
			epCache.invalidate(id)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"monitored": req.Monitored})
	}
}

func handleResyncMapping(engine *sync.Engine, appCtx context.Context, bgWG *stdsync.WaitGroup, epCache *episodeCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		// Claim the run slot before answering so a lost race is reported
		// as 409 here instead of a 202 for a sync that instantly no-ops.
		done, ok := engine.TryAcquireRun()
		if !ok {
			writeError(w, http.StatusConflict, "sync already in progress")
			return
		}
		bgWG.Add(1)
		go func() {
			defer bgWG.Done()
			defer done()
			if err := engine.RunResyncClaimed(appCtx, id); err != nil {
				log.Error().Err(err).Int64("mapping_id", id).Msg("resync failed")
			}
			epCache.invalidate(id)
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "resync triggered"})
	}
}

func handleTriggerSync(engine *sync.Engine, appCtx context.Context, bgWG *stdsync.WaitGroup, epCache *episodeCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		done, ok := engine.TryAcquireRun()
		if !ok {
			writeError(w, http.StatusConflict, "sync already in progress")
			return
		}
		bgWG.Add(1)
		go func() {
			defer bgWG.Done()
			defer done()
			if err := engine.RunSyncClaimed(appCtx, "manual"); err != nil {
				log.Error().Err(err).Msg("manual sync failed")
			}
			epCache.invalidateAll()
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync triggered"})
	}
}
