package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mizu/internal/config"
	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
	"mizu/internal/notifier"
	"mizu/internal/scraper"
	"mizu/internal/sonarr"

	"github.com/rs/zerolog/log"
)

type Engine struct {
	cfg      *config.Config
	db       *database.DB
	sonarr   *sonarr.Client
	scraper  *scraper.Scraper
	matcher  *matcherpkg.Matcher
	notifier *notifier.Notifier
	running  atomic.Bool
	mu       sync.RWMutex
}

type processSeriesResult struct {
	EpisodesSeen     int
	EpisodesUpdated  int
	MonitoredCount   int
	UnmonitoredCount int
	FillerFound      int
	MixedCanonFound  int
	AnimeCanonFound  int
}

func New(cfg *config.Config, db *database.DB) *Engine {
	e := &Engine{
		cfg:     cfg,
		db:      db,
		sonarr:  sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, cfg.RequestTimeout),
		scraper: scraper.New(cfg.RequestTimeout, cfg.RateLimitDelay, db),
		matcher: matcherpkg.New(0.82),
	}
	if cfg.NtfyEnabled {
		e.notifier = notifier.New(cfg.NtfyServerURL, cfg.NtfyTopic, cfg.NtfyToken)
	}
	return e
}

func (e *Engine) SonarrClient() *sonarr.Client {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sonarr
}
func (e *Engine) ScraperClient() *scraper.Scraper    { return e.scraper }
func (e *Engine) MatcherClient() *matcherpkg.Matcher { return e.matcher }
func (e *Engine) DB() *database.DB                   { return e.db }
func (e *Engine) IsRunning() bool                    { return e.running.Load() }

// SetRunning is a test helper to force the running flag. Production code
// should use CompareAndSwap via RunSync/RunResync instead.
func (e *Engine) SetRunning(v bool)                  { e.running.Store(v) }

// sonarrClient returns the current Sonarr client under a read lock.
func (e *Engine) sonarrClient() *sonarr.Client {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sonarr
}

// notifierClient returns the current notifier under a read lock.
func (e *Engine) notifierClient() *notifier.Notifier {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.notifier
}

// UpdateClients recreates the Sonarr and ntfy clients from the current config
// values. This should be called after settings are updated at runtime.
func (e *Engine) UpdateClients() {
	e.cfg.RLock()
	sonarrURL := e.cfg.SonarrURL
	sonarrAPIKey := e.cfg.SonarrAPIKey
	requestTimeout := e.cfg.RequestTimeout
	ntfyEnabled := e.cfg.NtfyEnabled
	ntfyServerURL := e.cfg.NtfyServerURL
	ntfyTopic := e.cfg.NtfyTopic
	ntfyToken := e.cfg.NtfyToken
	e.cfg.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	e.sonarr = sonarr.New(sonarrURL, sonarrAPIKey, requestTimeout)
	if ntfyEnabled {
		e.notifier = notifier.New(ntfyServerURL, ntfyTopic, ntfyToken)
	} else {
		e.notifier = nil
	}
}

func (e *Engine) RunSync(ctx context.Context, trigger string) error {
	if !e.running.CompareAndSwap(false, true) {
		return fmt.Errorf("sync already in progress")
	}
	e.cfg.RLock()
	apiKey := e.cfg.SonarrAPIKey
	e.cfg.RUnlock()
	if apiKey == "" {
		e.running.Store(false)
		log.Warn().Msg("skipping sync: no Sonarr API key configured")
		return fmt.Errorf("sonarr API key not configured")
	}

	defer func() { e.running.Store(false) }()

	log.Info().Str("trigger", trigger).Msg("starting sync cycle")
	startTime := time.Now()
	logID, err := e.db.CreateSyncLog(ctx, trigger)
	if err != nil {
		log.Error().Err(err).Msg("failed to create sync log")
	}

	mappings, err := e.db.GetAllMappings(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("failed to load mappings from database: %v", err)
		log.Error().Err(err).Msg(errMsg)
		e.completeSyncLog(ctx, logID, "failed", 0, 0, errMsg)
		return fmt.Errorf("failed to load mappings from database: %w", err)
	}

	enabledMappings := make([]models.ShowMapping, 0, len(mappings))
	for _, m := range mappings {
		if m.Enabled {
			enabledMappings = append(enabledMappings, m)
		}
	}
	log.Info().Int("mappings", len(enabledMappings)).Msg("syncing confirmed mappings")
	if len(enabledMappings) == 0 {
		e.completeSyncLog(ctx, logID, "success", 0, 0, "")
		return nil
	}

	// Process mappings concurrently with a small worker pool. The scraper's
	// rate limiter ensures AFL requests stay spaced out, while Sonarr API
	// calls overlap — cutting wall-clock time roughly 3x for large libraries.
	results := e.processMappingsConcurrent(ctx, enabledMappings)

	showsProcessed := 0
	episodesUpdated := 0
	var syncErrors []string
	var totalFiller, totalMixedCanon, totalAnimeCanon int
	runItems := make([]models.SyncLogItem, 0, len(enabledMappings))
	for _, res := range results {
		if res.fetchErr != nil {
			errMsg := fmt.Sprintf("failed to fetch Sonarr series %d (%s): %v", res.mapping.SonarrSeriesID, res.mapping.SonarrTitle, res.fetchErr)
			log.Error().Err(res.fetchErr).Msg(errMsg)
			syncErrors = append(syncErrors, errMsg)
			runItems = append(runItems, models.SyncLogItem{
				MappingID:       res.mapping.ID,
				SonarrTitle:     res.mapping.SonarrTitle,
				Status:          "failed",
				EpisodesChanged: 0,
				Summary:         "Failed before sync could start",
				Error:           res.fetchErr.Error(),
			})
			continue
		}
		if res.procErr != nil {
			errMsg := fmt.Sprintf("error processing %q: %v", res.mapping.SonarrTitle, res.procErr)
			log.Error().Err(res.procErr).Str("series", res.mapping.SonarrTitle).Msg("failed to process series")
			syncErrors = append(syncErrors, errMsg)
			runItems = append(runItems, models.SyncLogItem{
				MappingID:       res.mapping.ID,
				SonarrTitle:     res.mapping.SonarrTitle,
				Status:          "failed",
				EpisodesChanged: 0,
				Summary:         fmt.Sprintf("Checked %d episode%s before failing", res.result.EpisodesSeen, pluralize(res.result.EpisodesSeen)),
				Error:           res.procErr.Error(),
			})
			continue
		}
		showsProcessed++
		episodesUpdated += res.result.EpisodesUpdated
		totalFiller += res.result.FillerFound
		totalMixedCanon += res.result.MixedCanonFound
		totalAnimeCanon += res.result.AnimeCanonFound
		runItems = append(runItems, models.SyncLogItem{
			MappingID:       res.mapping.ID,
			SonarrTitle:     res.mapping.SonarrTitle,
			Status:          "success",
			EpisodesChanged: res.result.EpisodesUpdated,
			Summary:         summarizeProcessResult(res.result),
		})
	}

	status := "success"
	if len(syncErrors) > 0 && showsProcessed == 0 {
		status = "failed"
	} else if len(syncErrors) > 0 {
		status = "partial"
	}
	errorsStr := strings.Join(syncErrors, "; ")
	if err := e.db.ReplaceSyncLogItems(ctx, logID, runItems); err != nil {
		log.Error().Err(err).Int64("log_id", logID).Msg("failed to store sync log items")
	}
	e.completeSyncLog(ctx, logID, status, showsProcessed, episodesUpdated, errorsStr)

	log.Info().Str("status", status).Int("shows", showsProcessed).Int("episodes_updated", episodesUpdated).Int("errors", len(syncErrors)).Msg("sync cycle complete")
	if n := e.notifierClient(); n != nil && n.Enabled() {
		result := notifier.SyncResult{
			Status:          status,
			ShowsProcessed:  showsProcessed,
			EpisodesUpdated: episodesUpdated,
			FillerFound:     totalFiller,
			MixedCanonFound: totalMixedCanon,
			AnimeCanonFound: totalAnimeCanon,
			Errors:          syncErrors,
			Duration:        time.Since(startTime),
			Trigger:         trigger,
		}
		if err := n.SendSyncResult(ctx, result); err != nil {
			log.Warn().Err(err).Msg("failed to send ntfy notification")
		}
	}
	return nil
}

// mappingResult holds the outcome of processing a single mapping.
type mappingResult struct {
	mapping  models.ShowMapping
	series   *models.SonarrSeries
	result   processSeriesResult
	fetchErr error
	procErr  error
}

// processMappingsConcurrent runs mappings through a worker pool. The worker
// count is capped at 3 — enough to overlap Sonarr API calls (which are the
// main latency source) while keeping AFL scraper concurrency at 1 (the rate
// limiter serializes those anyway). Results are returned in input order so
// sync log items are deterministic.
func (e *Engine) processMappingsConcurrent(ctx context.Context, mappings []models.ShowMapping) []mappingResult {
	const maxWorkers = 3
	results := make([]mappingResult, len(mappings))

	jobs := make(chan int, len(mappings))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range jobs {
			if err := ctx.Err(); err != nil {
				results[idx] = mappingResult{
					mapping:  mappings[idx],
					fetchErr: err,
				}
				continue
			}
			res := mappingResult{mapping: mappings[idx]}
			series, err := e.sonarrClient().GetSeries(ctx, mappings[idx].SonarrSeriesID)
			if err != nil {
				res.fetchErr = err
				results[idx] = res
				continue
			}
			res.series = series
			procResult, err := e.processSeries(ctx, *series, &mappings[idx], false)
			res.result = procResult
			res.procErr = err
			results[idx] = res
		}
	}

	nWorkers := maxWorkers
	if nWorkers > len(mappings) {
		nWorkers = len(mappings)
	}
	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go worker()
	}
	for i := range mappings {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

func (e *Engine) RunResync(ctx context.Context, mappingID int64) error {
	if !e.running.CompareAndSwap(false, true) {
		return fmt.Errorf("sync already in progress")
	}
	defer func() { e.running.Store(false) }()

	e.cfg.RLock()
	apiKey := e.cfg.SonarrAPIKey
	e.cfg.RUnlock()
	if apiKey == "" {
		return fmt.Errorf("sonarr API key not configured")
	}

	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil || mapping == nil {
		return fmt.Errorf("mapping not found")
	}

	series, err := e.sonarrClient().GetSeries(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return fmt.Errorf("fetching Sonarr series: %w", err)
	}

	result, err := e.processSeries(ctx, *series, mapping, true)
	if err != nil {
		return fmt.Errorf("processing series: %w", err)
	}

	log.Info().Str("series", mapping.SonarrTitle).Int("episodes_updated", result.EpisodesUpdated).Msg("resync complete")
	return nil
}

func (e *Engine) InitialWatermarkForSeries(ctx context.Context, seriesID int) (int, error) {
	sonarrEpisodes, err := e.sonarrClient().GetEpisodes(ctx, seriesID)
	if err != nil {
		return 0, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	return initialWatermarkForEpisodes(sonarrEpisodes, time.Now()), nil
}

func (e *Engine) InitialWatermarkForStartMode(ctx context.Context, seriesID int, startMode models.StartMode) (int, error) {
	if normalizeStartMode(startMode) == models.StartModeFromBeginning {
		return 0, nil
	}
	return e.InitialWatermarkForSeries(ctx, seriesID)
}

func (e *Engine) SuggestMatches(ctx context.Context, seriesID int) (*models.SonarrSeries, []matcherpkg.MatchResult, error) {
	series, err := e.sonarrClient().GetSeries(ctx, seriesID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching Sonarr series: %w", err)
	}

	aflShows, err := e.scraper.GetAllShows(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("loading AFL shows: %w", err)
	}

	matches := e.matcher.FindMatches(ctx, *series, aflShows)
	return series, matches, nil
}

func (e *Engine) AutoMatchSeries(ctx context.Context, seriesID int) (*models.ShowMapping, error) {
	series, matches, err := e.SuggestMatches(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	return e.autoMatch(ctx, *series, matches[0])
}

func (e *Engine) HydrateMapping(ctx context.Context, mappingID int64) (*models.ShowMapping, error) {
	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return nil, fmt.Errorf("loading mapping: %w", err)
	}
	if mapping == nil {
		return nil, fmt.Errorf("mapping not found")
	}

	series, err := e.sonarrClient().GetSeries(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching Sonarr series: %w", err)
	}

	if _, err := e.processSeries(ctx, *series, mapping, false); err != nil {
		return nil, err
	}

	return e.db.GetMappingByID(ctx, mappingID)
}

func (e *Engine) processSeries(ctx context.Context, series models.SonarrSeries, mapping *models.ShowMapping, resyncAll bool) (processSeriesResult, error) {
	mapping.StartMode = normalizeStartMode(mapping.StartMode)
	mapping.WatermarkMode = normalizeWatermarkMode(mapping.WatermarkMode, mapping.StartMode)

	aflTitle, aflEpisodes, err := e.scraper.GetShowEpisodes(ctx, mapping.AFLSlug)
	if err != nil {
		return processSeriesResult{}, fmt.Errorf("fetching AFL episodes: %w", err)
	}

	if mapping.AFLTitle == "" && aflTitle != "" {
		mapping.AFLTitle = aflTitle
		if uerr := e.db.UpsertMapping(ctx, mapping); uerr != nil {
			log.Warn().Err(uerr).Msg("failed to backfill AFL title")
		}
	}

	aflByNumber, aflPreferred := BuildAFLEpisodeLookup(aflEpisodes)
	coverageMax := AFLCoverageMaxEpisodeNumber(aflEpisodes)

	sonarrEpisodes, err := e.sonarrClient().GetEpisodes(ctx, series.ID)
	if err != nil {
		return processSeriesResult{}, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}

	watermarkChanged, effectiveWatermark := e.refreshWatermark(ctx, mapping, sonarrEpisodes)
	effectiveWatermark = clampWatermarkToCoverage(effectiveWatermark, coverageMax)
	if watermarkChanged {
		log.Info().Str("series", series.Title).Int("watermark", effectiveWatermark).Str("mode", string(mapping.WatermarkMode)).Msg("updated sync watermark")
	}

	var toUnmonitor []int
	var toMonitor []int
	dbEpisodes := make([]models.EpisodeInfo, 0, len(sonarrEpisodes))
	result := processSeriesResult{}

	for _, ep := range sonarrEpisodes {
		aflNum, aflEpisode, found := SelectAFLEpisodeForSonarr(ep, aflByNumber, aflPreferred)
		if !IsWithinAFLCoverage(ep, aflNum, found, coverageMax) {
			continue
		}
		result.EpisodesSeen++

		epType := models.EpisodeUnknown
		title := ep.Title
		if found {
			epType = aflEpisode.Type
			if aflEpisode.Title != "" {
				title = aflEpisode.Title
			}
		}

		switch epType {
		case models.EpisodeFiller:
			result.FillerFound++
		case models.EpisodeMixedCanon:
			result.MixedCanonFound++
		case models.EpisodeAnimeCanon:
			result.AnimeCanonFound++
		}

		if !isSpecialSeason(ep.SeasonNumber) {
			dbEpisodes = append(dbEpisodes, models.EpisodeInfo{
				MappingID:             mapping.ID,
				EpisodeNum:            aflNum,
				Title:                 title,
				Type:                  epType,
				SonarrEpisodeID:       ep.ID,
				SeasonNumber:          ep.SeasonNumber,
				SonarrEpisodeNumber:   ep.EpisodeNumber,
				AbsoluteEpisodeNumber: ep.AbsoluteEpisodeNumber,
				Monitored:             ep.Monitored,
			})
		}

		if isBelowOrAtWatermark(aflNum, effectiveWatermark) {
			if ep.Monitored {
				toUnmonitor = append(toUnmonitor, ep.ID)
			}
			continue
		}

		if e.shouldMonitor(epType, mapping) {
			if !ep.Monitored {
				toMonitor = append(toMonitor, ep.ID)
			}
			continue
		}

		if ep.Monitored {
			toUnmonitor = append(toUnmonitor, ep.ID)
		}
	}

	if err := e.db.ReplaceEpisodesForMapping(ctx, mapping.ID, dbEpisodes); err != nil {
		log.Warn().Err(err).Str("series", series.Title).Msg("failed to cache episode data")
	}

	if len(toUnmonitor) > 0 {
		if err := e.sonarrClient().BulkSetEpisodesMonitored(ctx, toUnmonitor, false); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("failed to unmonitor episodes")
		} else {
			result.UnmonitoredCount = len(toUnmonitor)
			result.EpisodesUpdated += len(toUnmonitor)
			log.Info().Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("unmonitored episodes")
		}
	}
	if len(toMonitor) > 0 {
		if err := e.sonarrClient().BulkSetEpisodesMonitored(ctx, toMonitor, true); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toMonitor)).Msg("failed to monitor episodes")
		} else {
			result.MonitoredCount = len(toMonitor)
			result.EpisodesUpdated += len(toMonitor)
			log.Info().Str("series", series.Title).Int("count", len(toMonitor)).Msg("monitored episodes")
		}
	}

	if resyncAll && mapping.WatermarkMode == models.WatermarkModeAuto {
		log.Debug().Str("series", series.Title).Int("watermark", effectiveWatermark).Msg("resync respected auto watermark boundary")
	}

	return result, nil
}

func summarizeProcessResult(result processSeriesResult) string {
	parts := []string{fmt.Sprintf("Checked %d episode%s", result.EpisodesSeen, pluralize(result.EpisodesSeen))}
	if result.MonitoredCount > 0 {
		parts = append(parts, fmt.Sprintf("monitored %d", result.MonitoredCount))
	}
	if result.UnmonitoredCount > 0 {
		parts = append(parts, fmt.Sprintf("unmonitored %d", result.UnmonitoredCount))
	}
	if result.EpisodesUpdated == 0 {
		parts = append(parts, "no monitor changes")
	}
	return strings.Join(parts, " • ")
}

func preferredAFLEpisode(current, candidate models.AFLEpisode) models.AFLEpisode {
	if episodeTypePriority(candidate.Type) > episodeTypePriority(current.Type) {
		return candidate
	}
	if episodeTypePriority(candidate.Type) < episodeTypePriority(current.Type) {
		return current
	}
	if current.Title == "" && candidate.Title != "" {
		return candidate
	}
	return current
}

func BuildAFLEpisodeLookup(aflEpisodes []models.AFLEpisode) (map[int][]models.AFLEpisode, map[int]models.AFLEpisode) {
	aflPreferred := make(map[int]models.AFLEpisode, len(aflEpisodes))
	aflByNumber := make(map[int][]models.AFLEpisode, len(aflEpisodes))
	for _, ep := range aflEpisodes {
		aflByNumber[ep.Number] = append(aflByNumber[ep.Number], ep)
		if existing, ok := aflPreferred[ep.Number]; ok {
			aflPreferred[ep.Number] = preferredAFLEpisode(existing, ep)
			continue
		}
		aflPreferred[ep.Number] = ep
	}
	return aflByNumber, aflPreferred
}

func AFLCoverageMaxEpisodeNumber(aflEpisodes []models.AFLEpisode) int {
	maxEpisode := 0
	for _, ep := range aflEpisodes {
		if ep.Number > maxEpisode {
			maxEpisode = ep.Number
		}
	}
	return maxEpisode
}

func IsWithinAFLCoverage(sonarrEp models.SonarrEpisode, aflNum int, found bool, coverageMax int) bool {
	if aflNum <= 0 {
		return false
	}
	if isSpecialSeason(sonarrEp.SeasonNumber) {
		return found
	}
	if coverageMax <= 0 {
		return false
	}
	return aflNum <= coverageMax
}

func SelectAFLEpisodeForSonarr(sonarrEp models.SonarrEpisode, aflByNumber map[int][]models.AFLEpisode, aflPreferred map[int]models.AFLEpisode) (int, models.AFLEpisode, bool) {
	aflNum := matcherpkg.MapEpisodeNumber(sonarrEp, 0)
	if aflNum <= 0 {
		if isSpecialSeason(sonarrEp.SeasonNumber) {
			if matched, ok := matchAFLEpisodeByTitleAcrossLookupStrict(sonarrEp.Title, aflByNumber); ok {
				return matched.Number, matched, true
			}
		}
		return 0, models.AFLEpisode{}, false
	}

	candidates := aflByNumber[aflNum]
	if len(candidates) == 0 {
		if isSpecialSeason(sonarrEp.SeasonNumber) {
			if matched, ok := matchAFLEpisodeByTitleAcrossLookupStrict(sonarrEp.Title, aflByNumber); ok {
				return matched.Number, matched, true
			}
		}
		return aflNum, models.AFLEpisode{}, false
	}

	if isSpecialSeason(sonarrEp.SeasonNumber) {
		if matched, ok := matchAFLEpisodeByTitleStrict(sonarrEp.Title, candidates); ok {
			return aflNum, matched, true
		}
		if matched, ok := matchAFLEpisodeByTitleAcrossLookupStrict(sonarrEp.Title, aflByNumber); ok {
			return matched.Number, matched, true
		}
		return aflNum, models.AFLEpisode{}, false
	}

	if len(candidates) == 1 {
		return aflNum, candidates[0], true
	}

	if matched, ok := matchAFLEpisodeByTitle(sonarrEp.Title, candidates); ok {
		return aflNum, matched, true
	}

	matched, ok := aflPreferred[aflNum]
	return aflNum, matched, ok
}

func matchAFLEpisodeByTitleStrict(title string, candidates []models.AFLEpisode) (models.AFLEpisode, bool) {
	normalizer := matcherpkg.New(0)
	normalizedTitle := normalizer.Normalize(title)
	if normalizedTitle == "" {
		return models.AFLEpisode{}, false
	}

	bestScore := 0
	best := models.AFLEpisode{}
	for _, candidate := range candidates {
		normalizedCandidate := normalizer.Normalize(candidate.Title)
		if normalizedCandidate == "" {
			continue
		}

		score := 0
		switch {
		case normalizedCandidate == normalizedTitle:
			score = 4
		case strings.Contains(normalizedTitle, normalizedCandidate) || strings.Contains(normalizedCandidate, normalizedTitle):
			score = 3
		}

		if score > bestScore || (score == bestScore && score > 0 && episodeTypePriority(candidate.Type) > episodeTypePriority(best.Type)) {
			best = candidate
			bestScore = score
		}
	}

	return best, bestScore > 0
}

func matchAFLEpisodeByTitleAcrossLookupStrict(title string, aflByNumber map[int][]models.AFLEpisode) (models.AFLEpisode, bool) {
	allCandidates := make([]models.AFLEpisode, 0)
	for _, candidates := range aflByNumber {
		allCandidates = append(allCandidates, candidates...)
	}
	return matchAFLEpisodeByTitleStrict(title, allCandidates)
}

func matchAFLEpisodeByTitle(title string, candidates []models.AFLEpisode) (models.AFLEpisode, bool) {
	normalizer := matcherpkg.New(0)
	normalizedTitle := normalizer.Normalize(title)
	if normalizedTitle == "" {
		return models.AFLEpisode{}, false
	}

	bestScore := 0
	best := models.AFLEpisode{}
	for _, candidate := range candidates {
		normalizedCandidate := normalizer.Normalize(candidate.Title)
		if normalizedCandidate == "" {
			continue
		}

		score := 0
		switch {
		case normalizedCandidate == normalizedTitle:
			score = 4
		case strings.Contains(normalizedTitle, normalizedCandidate) || strings.Contains(normalizedCandidate, normalizedTitle):
			score = 3
		default:
			shared := sharedNormalizedTokenCount(normalizedTitle, normalizedCandidate)
			if shared > 1 {
				score = 2
			}
		}

		if score > bestScore || (score == bestScore && score > 0 && episodeTypePriority(candidate.Type) > episodeTypePriority(best.Type)) {
			best = candidate
			bestScore = score
		}
	}

	return best, bestScore > 0
}

func sharedNormalizedTokenCount(a, b string) int {
	if a == "" || b == "" {
		return 0
	}

	seen := make(map[string]struct{})
	for _, token := range strings.Fields(a) {
		if len(token) < 2 {
			continue
		}
		seen[token] = struct{}{}
	}

	count := 0
	used := make(map[string]struct{})
	for _, token := range strings.Fields(b) {
		if len(token) < 2 {
			continue
		}
		if _, ok := used[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			count++
			used[token] = struct{}{}
		}
	}

	return count
}

func episodeTypePriority(t models.EpisodeType) int {
	switch t {
	case models.EpisodeMixedCanon:
		return 4
	case models.EpisodeAnimeCanon:
		return 3
	case models.EpisodeCanon:
		return 2
	case models.EpisodeFiller:
		return 1
	default:
		return 0
	}
}

func isSpecialSeason(season int) bool {
	return season == 0 || season == -1
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (e *Engine) shouldMonitor(epType models.EpisodeType, mapping *models.ShowMapping) bool {
	switch epType {
	case models.EpisodeCanon:
		return true
	case models.EpisodeAnimeCanon:
		return mapping.AnimeCanonAction == models.ActionMonitor
	case models.EpisodeMixedCanon:
		return mapping.MixedAction == models.ActionMonitor
	case models.EpisodeFiller:
		return mapping.FillerAction == models.ActionMonitor
	default:
		return true
	}
}

func normalizeStartMode(mode models.StartMode) models.StartMode {
	if mode == models.StartModeFromBeginning {
		return mode
	}
	return models.StartModeCurrentProgress
}

func normalizeWatermarkMode(mode models.WatermarkMode, startMode models.StartMode) models.WatermarkMode {
	if mode == models.WatermarkModeManual || mode == models.WatermarkModeAuto {
		return mode
	}
	if normalizeStartMode(startMode) == models.StartModeFromBeginning {
		return models.WatermarkModeManual
	}
	return models.WatermarkModeAuto
}

func (e *Engine) refreshWatermark(ctx context.Context, mapping *models.ShowMapping, episodes []models.SonarrEpisode) (bool, int) {
	mapping.WatermarkMode = normalizeWatermarkMode(mapping.WatermarkMode, mapping.StartMode)
	if mapping.SyncWatermark < 0 {
		mapping.SyncWatermark = 0
	}
	if mapping.WatermarkMode != models.WatermarkModeAuto {
		return false, mapping.SyncWatermark
	}

	computed := nextAutoWatermark(mapping.SyncWatermark, episodes, time.Now())
	if computed == mapping.SyncWatermark {
		return false, computed
	}

	mapping.SyncWatermark = computed
	if err := e.db.UpdateWatermarkState(ctx, mapping.ID, computed, mapping.WatermarkMode); err != nil {
		log.Warn().Err(err).Int64("mapping_id", mapping.ID).Msg("failed to persist auto watermark")
		return false, mapping.SyncWatermark
	}
	return true, computed
}

func isBelowOrAtWatermark(episodeNum, watermark int) bool {
	if watermark <= 0 {
		return false
	}
	return episodeNum <= watermark
}

func nextAutoWatermark(current int, episodes []models.SonarrEpisode, now time.Time) int {
	computed := initialWatermarkForEpisodes(episodes, now)
	if computed < current {
		return current
	}
	return computed
}

func initialWatermarkForEpisodes(episodes []models.SonarrEpisode, now time.Time) int {
	maxWatermark := 0
	for _, ep := range episodes {
		if isSpecialSeason(ep.SeasonNumber) {
			continue
		}
		if ep.AirDateUTC.After(now) {
			continue
		}
		if !ep.HasFile {
			continue
		}
		aflNum := matcherpkg.MapEpisodeNumber(ep, 0)
		if aflNum > maxWatermark {
			maxWatermark = aflNum
		}
	}
	return maxWatermark
}

func clampWatermarkToCoverage(watermark, coverageMax int) int {
	if coverageMax <= 0 {
		return watermark
	}
	if watermark > coverageMax {
		return coverageMax
	}
	return watermark
}

func (e *Engine) completeSyncLog(ctx context.Context, logID int64, status string, showsProcessed, episodesUpdated int, errors string) {
	if logID == 0 {
		return
	}
	if err := e.db.CompleteSyncLog(ctx, logID, status, showsProcessed, episodesUpdated, errors); err != nil {
		log.Error().Err(err).Msg("failed to complete sync log")
	}
}

func (e *Engine) autoMatch(ctx context.Context, series models.SonarrSeries, bestMatch matcherpkg.MatchResult) (*models.ShowMapping, error) {
	log.Info().Str("sonarr", series.Title).Str("afl", bestMatch.AFLShow.Title).Float64("confidence", bestMatch.Confidence).Str("method", bestMatch.Method).Msg("auto-matched series")

	initialWatermark, err := e.InitialWatermarkForStartMode(ctx, series.ID, models.StartModeCurrentProgress)
	if err != nil {
		return nil, fmt.Errorf("computing initial watermark: %w", err)
	}

	e.cfg.RLock()
	defaultFiller := e.cfg.DefaultFillerAction
	defaultMixed := e.cfg.DefaultMixedCanonAction
	defaultAnimeCanon := e.cfg.DefaultAnimeCanonAction
	e.cfg.RUnlock()

	mapping := &models.ShowMapping{
		SonarrSeriesID:   series.ID,
		SonarrTitle:      series.Title,
		AFLSlug:          bestMatch.AFLShow.Slug,
		AFLTitle:         bestMatch.AFLShow.Title,
		AutoMatched:      true,
		MatchConfidence:  bestMatch.Confidence,
		FillerAction:     models.FillerAction(defaultFiller),
		MixedAction:      models.FillerAction(defaultMixed),
		AnimeCanonAction: models.FillerAction(defaultAnimeCanon),
		Enabled:          true,
		StartMode:        models.StartModeCurrentProgress,
		SyncWatermark:    initialWatermark,
		CreatedAt:        time.Now(),
	}

	if err := e.db.UpsertMapping(ctx, mapping); err != nil {
		return nil, fmt.Errorf("saving auto-match: %w", err)
	}
	return mapping, nil
}
