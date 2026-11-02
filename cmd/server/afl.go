package main

import (
	"context"
	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/metrics"
	"mizu/internal/models"
	"mizu/internal/scraper"
	"mizu/internal/sync"
	"net/http"
	"sort"
	"strconv"
	"strings"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// aflCacheRefreshing prevents concurrent AFL cache refreshes — two
// overlapping scrapes would double-hammer AnimeFillerList for no benefit.
var aflCacheRefreshing atomic.Bool

// aflCacheLastError holds the failure message from the most recent
// background cache refresh, surfaced via the status endpoint so the UI
// can report failures that no longer have an HTTP response to ride on.
var aflCacheLastError atomic.Pointer[string]

func handleRefreshAFLCache(engine *sync.Engine, db *database.DB, appCtx context.Context, bgWG *stdsync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !aflCacheRefreshing.CompareAndSwap(false, true) {
			writeError(w, http.StatusConflict, "AFL cache refresh already in progress")
			return
		}
		// Ownership of the CAS flag transfers to the goroutine — it clears
		// the flag when the scrape finishes. The scrape runs on the app
		// context (not the request) so it survives client disconnects and
		// the server's WriteTimeout, which a slow or retried fetch can
		// outlive.
		bgWG.Add(1)
		go func() {
			defer bgWG.Done()
			defer aflCacheRefreshing.Store(false)
			runAFLCacheRefresh(appCtx, engine, db)
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "refreshing"})
	}
}

// aflCacheCheckInterval is how often the background refresher checks
// whether the cached show list is due. A failed refresh is retried on the
// next check.
const aflCacheCheckInterval = time.Hour

// aflCacheRefreshDue reports whether the cached show list should be
// fetched again: it is empty or at least ShowListRefreshInterval old.
func aflCacheRefreshDue(count int, refreshedAt, now time.Time) bool {
	return count == 0 || refreshedAt.IsZero() || now.Sub(refreshedAt) >= scraper.ShowListRefreshInterval
}

// refreshAFLCacheIfDue runs a refresh when the cache is due and no other
// refresh is running, reporting whether it ran one.
func refreshAFLCacheIfDue(ctx context.Context, engine *sync.Engine, db *database.DB, bgWG *stdsync.WaitGroup) bool {
	count, refreshedAt, err := db.GetAFLShowCacheStatus(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to read AFL cache status")
		return false
	}
	// Skip once shutdown has begun, so bgWG isn't added to while main waits on it.
	if ctx.Err() != nil || !aflCacheRefreshDue(count, refreshedAt, time.Now()) || !aflCacheRefreshing.CompareAndSwap(false, true) {
		return false
	}
	bgWG.Add(1)
	defer bgWG.Done()
	defer aflCacheRefreshing.Store(false)
	log.Info().Int("cached", count).Msg("AFL show list is due, refreshing")
	runAFLCacheRefresh(ctx, engine, db)
	return true
}

// startAFLCacheRefresher keeps the AFL show list current, so new titles
// reach the add dialog and new-title alerts without a manual refresh. It
// checks shortly after startup and then every aflCacheCheckInterval.
func startAFLCacheRefresher(ctx context.Context, engine *sync.Engine, db *database.DB, bgWG *stdsync.WaitGroup) {
	first := time.NewTimer(catchUpSyncDelay)
	defer first.Stop()
	ticker := time.NewTicker(aflCacheCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-first.C:
		case <-ticker.C:
		}
		refreshAFLCacheIfDue(ctx, engine, db, bgWG)
	}
}

func handleAFLCacheRefreshStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{"refreshing": aflCacheRefreshing.Load()}
	if errPtr := aflCacheLastError.Load(); errPtr != nil {
		resp["last_error"] = *errPtr
	}
	writeJSON(w, http.StatusOK, resp)
}

// runAFLCacheRefresh performs the actual scrape and records the outcome
// in the activity feed. It runs in the background: failures can't be
// returned over HTTP, so they land in the sync log (status "failed")
// and in aflCacheLastError for the status endpoint.
func runAFLCacheRefresh(ctx context.Context, engine *sync.Engine, db *database.DB) {
	aflCacheLastError.Store(nil)

	shows, newSlugs, err := engine.ScraperClient().RefreshAllShows(ctx)
	if err != nil {
		msg := err.Error()
		aflCacheLastError.Store(&msg)
		metrics.Add("mizu_afl_cache_refresh_total", "failed")
		log.Error().Err(err).Msg("AFL cache refresh failed")
		// WithoutCancel so a shutdown-cancelled scrape still leaves a
		// failed log row instead of an orphaned "running" entry.
		logCtx := context.WithoutCancel(ctx)
		if logID, logErr := db.CreateSyncLog(logCtx, "afl_cache_refresh"); logErr == nil {
			if cerr := db.CompleteSyncLog(logCtx, logID, "failed", 0, 0, msg); cerr != nil {
				log.Warn().Err(cerr).Int64("log_id", logID).Msg("failed to complete AFL cache refresh log")
			}
		} else {
			log.Warn().Err(logErr).Msg("failed to create AFL cache refresh failure log")
		}
		return
	}

	// Newly discovered shows are logged (and announced) by the scraper's
	// new-shows hook, which covers automatic refreshes too. This row only
	// records the refresh itself, so skip it when the hook already wrote
	// one; a first fill (every show "new") is not reported by the hook.
	if len(newSlugs) == 0 || len(newSlugs) == len(shows) {
		logID, err := db.CreateSyncLog(ctx, "afl_cache_refresh")
		if err != nil {
			log.Warn().Err(err).Msg("failed to create AFL cache refresh log")
		} else if err := db.CompleteSyncLog(ctx, logID, "success", 0, 0, ""); err != nil {
			log.Warn().Err(err).Int64("log_id", logID).Msg("failed to complete AFL cache refresh log")
		}
	}

	count, _, err := db.GetAFLShowCacheStatus(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to read AFL cache status after refresh")
		return
	}
	if count == 0 {
		count = len(shows)
	}
	metrics.Add("mizu_afl_cache_refresh_total", "success")
	log.Info().Int("shows", count).Int("new_shows", len(newSlugs)).Msg("AFL cache refresh complete")
}

func handleAFLSearch(engine *sync.Engine, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		cachedShows, err := loadCachedAFLShows(r.Context(), db)
		if err != nil {
			writeInternalError(w, err, "failed to load cached AFL shows")
			return
		}

		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeInternalError(w, err, "failed to load mappings")
			return
		}

		seriesIDParam := r.URL.Query().Get("series_id")
		seriesID := 0
		if seriesIDParam != "" {
			var parseErr error
			seriesID, parseErr = strconv.Atoi(seriesIDParam)
			if parseErr != nil || seriesID == 0 {
				writeError(w, http.StatusBadRequest, "invalid series_id parameter")
				return
			}
		}

		results := excludeMappedAFLShows(filterCachedAFLShows(query, cachedShows), mappings, seriesID)

		if seriesID == 0 {
			writeJSON(w, http.StatusOK, results)
			return
		}

		series, err := engine.SonarrClient().GetSeries(r.Context(), seriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load Sonarr series: "+err.Error())
			return
		}

		matches := excludeMappedMatchResults(matchCachedAFLShows(r.Context(), engine.MatcherClient(), *series, results), mappings, seriesID)
		if len(matches) == 0 {
			writeJSON(w, http.StatusOK, results)
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
			writeInternalError(w, err, "failed to load cached AFL shows")
			return
		}

		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeInternalError(w, err, "failed to load mappings")
			return
		}

		series, err := engine.SonarrClient().GetSeries(r.Context(), seriesID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load Sonarr series: "+err.Error())
			return
		}

		matches := excludeMappedMatchResults(matchCachedAFLShows(r.Context(), engine.MatcherClient(), *series, cachedShows), mappings, seriesID)
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

func matchCachedAFLShows(ctx context.Context, m *matcherpkg.Matcher, series models.SonarrSeries, shows []models.AFLShow) []matcherpkg.MatchResult {
	return m.FindMatches(ctx, series, shows)
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
