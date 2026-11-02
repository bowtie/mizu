package main

import (
	"context"
	"encoding/json"
	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
	"mizu/internal/sync"
	"net/http"
	"sort"
	"strconv"
	"strings"
	stdsync "sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

const manualMatchPendingSeriesIDsKey = "manual_match_pending_series_ids"

// manualMatchMu guards the read-modify-write cycle on the
// manual_match_pending_series_ids setting. Without it, two concurrent
// requests could load the same list, each add/remove one ID, and the
// second write would clobber the first.
var manualMatchMu stdsync.Mutex

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
			writeInternalError(w, err, "failed to persist pending manual match")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "pending", "sonarr_series_id": req.SonarrSeriesID})
	}
}

func handleGetSonarrSeriesByID(db *database.DB, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid series ID")
			return
		}

		// Serve from cache if fresh (< 30 days). Sonarr series metadata
		// (title, overview, images) rarely changes, so a long TTL is fine.
		// Use ?force=true to bypass the cache and re-fetch from Sonarr.
		force := r.URL.Query().Get("force") == "true"
		if !force {
			if cached, ok := db.GetCachedSonarrSeries(r.Context(), id, 30*24*time.Hour); ok {
				writeJSON(w, http.StatusOK, cached.Public())
				return
			}
		}

		c := syncEngine.SonarrClient()
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
		if err := db.SetCachedSonarrSeries(r.Context(), id, series); err != nil {
			log.Warn().Err(err).Int("series_id", id).Msg("failed to cache Sonarr series")
		}

		// Also cache the overview on the mapping so the frontend can
		// display it without a separate Sonarr API call.
		if series.Overview != "" {
			if mapping, err := db.GetMappingBySonarrID(r.Context(), id); err == nil && mapping != nil {
				if mapping.Overview != series.Overview {
					if err := db.UpdateOverview(r.Context(), mapping.ID, series.Overview); err != nil {
						log.Warn().Err(err).Int64("mapping_id", mapping.ID).Msg("failed to cache overview")
					}
				}
			}
		}

		writeJSON(w, http.StatusOK, series.Public())
	}
}

func handleGetSonarrSeries(db *database.DB, engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchableOnly := r.URL.Query().Get("matchable_only") == "true"

		c := engine.SonarrClient()
		series, err := c.GetAnimeSeries(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr series: "+err.Error())
			return
		}

		if matchableOnly {
			mappings, err := db.GetAllMappings(r.Context())
			if err != nil {
				writeInternalError(w, err, "failed to load mappings")
				return
			}

			cachedShows, err := loadCachedAFLShows(r.Context(), db)
			if err != nil {
				writeInternalError(w, err, "failed to load cached AFL shows")
				return
			}

			pendingManualMatchIDs, err := loadManualMatchPendingSeriesIDs(r.Context(), db)
			if err != nil {
				writeInternalError(w, err, "failed to load pending manual matches")
				return
			}

			series = filterSeriesForAddModal(r.Context(), series, mappings, pendingManualMatchIDs, engine.MatcherClient(), cachedShows)
		}

		public := make([]models.SonarrSeriesPublic, len(series))
		for i := range series {
			public[i] = series[i].Public()
		}

		writeJSON(w, http.StatusOK, public)
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
	manualMatchMu.Lock()
	defer manualMatchMu.Unlock()
	ids, err := loadManualMatchPendingSeriesIDs(ctx, db)
	if err != nil {
		return err
	}
	ids[seriesID] = struct{}{}
	return storeManualMatchPendingSeriesIDs(ctx, db, ids)
}

func clearManualMatchPendingSeriesID(ctx context.Context, db *database.DB, seriesID int) error {
	manualMatchMu.Lock()
	defer manualMatchMu.Unlock()
	ids, err := loadManualMatchPendingSeriesIDs(ctx, db)
	if err != nil {
		return err
	}
	delete(ids, seriesID)
	return storeManualMatchPendingSeriesIDs(ctx, db, ids)
}
