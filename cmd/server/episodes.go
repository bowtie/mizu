package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func handleGetEpisodes(db *database.DB, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		episodes, herr := loadEpisodes(r.Context(), db, syncEngine, id, nil)
		if herr != nil {
			writeError(w, herr.code, herr.message)
			return
		}
		held, err := db.GetHeldEpisodes(r.Context(), id)
		if err != nil {
			log.Warn().Err(err).Int64("mapping_id", id).Msg("failed to load held episodes")
		}
		for i := range episodes {
			_, episodes[i].Held = held[episodes[i].SonarrEpisodeID]
		}
		writeJSONWithETag(w, r, episodes)
	}
}

// loadEpisodes returns a mapping's episodes from the database (populated by
// syncs), falling back to a live Sonarr + AFL fetch when the database has
// none yet. The mapping is loaded on demand when the caller doesn't have it.
func loadEpisodes(ctx context.Context, db *database.DB, syncEngine *sync.Engine, id int64, mapping *models.ShowMapping) ([]models.EpisodeInfo, *httpError) {
	dbEpisodes, err := db.GetEpisodes(ctx, id)
	if err != nil {
		log.Error().Err(err).Int64("mapping_id", id).Msg("failed to load episodes")
		return nil, &httpError{http.StatusInternalServerError, "failed to load episodes"}
	}
	if len(dbEpisodes) > 0 && hasSonarrEpisodeIDs(dbEpisodes) {
		return dbEpisodes, nil
	}

	if mapping == nil {
		mapping, err = db.GetMappingByID(ctx, id)
		if err != nil || mapping == nil {
			return nil, &httpError{http.StatusNotFound, "mapping not found"}
		}
	}
	if syncEngine == nil {
		return nil, &httpError{http.StatusInternalServerError, "sync engine unavailable"}
	}

	enriched, herr := fetchEpisodesLive(ctx, syncEngine, id, mapping.SonarrSeriesID, mapping.AFLSlug)
	if herr != nil {
		return nil, herr
	}
	// Persist so future loads are served from the database.
	if err := db.ReplaceEpisodesForMapping(ctx, id, enriched); err != nil {
		log.Warn().Err(err).Int64("mapping_id", id).Msg("failed to persist live-fetched episodes")
	}
	return enriched, nil
}

// writeJSONWithETag writes data as JSON with an ETag derived from the exact
// response bytes, answering 304 when the client already has them. Hashing
// the body (rather than a summary of the rows) means the tag can never
// describe different data than the body it was sent with.
func writeJSONWithETag(w http.ResponseWriter, r *http.Request, data interface{}) {
	body, err := json.Marshal(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:12]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(body, '\n'))
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

func fetchEpisodesLive(ctx context.Context, syncEngine *sync.Engine, mappingID int64, sonarrSeriesID int, aflSlug string) ([]models.EpisodeInfo, *httpError) {
	sonarrEpisodes, err := syncEngine.SonarrClient().GetEpisodes(ctx, sonarrSeriesID)
	if err != nil {
		return nil, &httpError{http.StatusBadGateway, "failed to fetch episodes from Sonarr: " + err.Error()}
	}

	_, aflEpisodes, err := syncEngine.ScraperClient().GetShowEpisodes(ctx, aflSlug)
	if err != nil {
		return nil, &httpError{http.StatusBadGateway, "failed to fetch AFL episodes: " + err.Error()}
	}

	sonarrEpisodes = sync.AlignToAFL(sonarrEpisodes, aflEpisodes)
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
			writeInternalError(w, err, "")
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

func handleSetEpisodeMonitor(db *database.DB, syncEngine *sync.Engine) http.HandlerFunc {
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
		// Mirror the change into the local DB so the episode list reflects it.
		if idErr == nil {
			_ = db.UpdateEpisodeMonitor(r.Context(), id, int64(epID), req.Monitored)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"monitored": req.Monitored})
	}
}

func handleResyncMapping(engine *sync.Engine, appCtx context.Context, bgWG *stdsync.WaitGroup) http.HandlerFunc {
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
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "resync triggered"})
	}
}

func handleTriggerSync(engine *sync.Engine, appCtx context.Context, bgWG *stdsync.WaitGroup) http.HandlerFunc {
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
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync triggered"})
	}
}
