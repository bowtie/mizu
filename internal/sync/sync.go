package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"mizu/internal/config"
	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
	"mizu/internal/notifier"
	"mizu/internal/scraper"
	"mizu/internal/sonarr"
)

type Engine struct {
	cfg      *config.Config
	db       *database.DB
	sonarr   *sonarr.Client
	scraper  *scraper.Scraper
	matcher  *matcherpkg.Matcher
	notifier *notifier.Notifier
	running  bool
}

type processSeriesResult struct {
	EpisodesSeen     int
	EpisodesUpdated  int
	MonitoredCount   int
	UnmonitoredCount int
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

func (e *Engine) SonarrClient() *sonarr.Client       { return e.sonarr }
func (e *Engine) ScraperClient() *scraper.Scraper    { return e.scraper }
func (e *Engine) MatcherClient() *matcherpkg.Matcher { return e.matcher }
func (e *Engine) DB() *database.DB                   { return e.db }
func (e *Engine) IsRunning() bool                    { return e.running }

func (e *Engine) RunSync(ctx context.Context, trigger string) error {
	if e.running {
		return fmt.Errorf("sync already in progress")
	}
	if e.cfg.SonarrAPIKey == "" {
		log.Warn().Msg("skipping sync: no Sonarr API key configured")
		return fmt.Errorf("sonarr API key not configured")
	}

	e.running = true
	defer func() { e.running = false }()

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
		return fmt.Errorf(errMsg)
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

	var fillerTagID int
	if e.cfg.EnableTagging {
		fillerTagID, err = e.sonarr.EnsureTag(ctx, e.cfg.FillerTagName)
		if err != nil {
			log.Warn().Err(err).Msg("failed to ensure filler tag, continuing without tagging")
		}
	}

	showsProcessed := 0
	episodesUpdated := 0
	var syncErrors []string
	runItems := make([]models.SyncLogItem, 0, len(enabledMappings))
	for _, mapping := range enabledMappings {
		series, err := e.sonarr.GetSeries(ctx, mapping.SonarrSeriesID)
		if err != nil {
			errMsg := fmt.Sprintf("failed to fetch Sonarr series %d (%s): %v", mapping.SonarrSeriesID, mapping.SonarrTitle, err)
			log.Error().Err(err).Msg(errMsg)
			syncErrors = append(syncErrors, errMsg)
			runItems = append(runItems, models.SyncLogItem{
				MappingID:       mapping.ID,
				SonarrTitle:     mapping.SonarrTitle,
				Status:          "failed",
				EpisodesChanged: 0,
				Summary:         "Failed before sync could start",
				Error:           err.Error(),
			})
			continue
		}

		result, err := e.processSeries(ctx, *series, &mapping, fillerTagID, false)
		if err != nil {
			errMsg := fmt.Sprintf("error processing %q: %v", mapping.SonarrTitle, err)
			log.Error().Err(err).Str("series", mapping.SonarrTitle).Msg("failed to process series")
			syncErrors = append(syncErrors, errMsg)
			runItems = append(runItems, models.SyncLogItem{
				MappingID:       mapping.ID,
				SonarrTitle:     mapping.SonarrTitle,
				Status:          "failed",
				EpisodesChanged: 0,
				Summary:         fmt.Sprintf("Checked %d episode%s before failing", result.EpisodesSeen, pluralize(result.EpisodesSeen)),
				Error:           err.Error(),
			})
			continue
		}
		showsProcessed++
		episodesUpdated += result.EpisodesUpdated
		runItems = append(runItems, models.SyncLogItem{
			MappingID:       mapping.ID,
			SonarrTitle:     mapping.SonarrTitle,
			Status:          "success",
			EpisodesChanged: result.EpisodesUpdated,
			Summary:         summarizeProcessResult(result),
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
	if e.notifier != nil && e.notifier.Enabled() {
		result := notifier.SyncResult{
			Status:          status,
			ShowsProcessed:  showsProcessed,
			EpisodesUpdated: episodesUpdated,
			Errors:          syncErrors,
			Duration:        time.Since(startTime),
			Trigger:         trigger,
		}
		if err := e.notifier.SendSyncResult(ctx, result); err != nil {
			log.Warn().Err(err).Msg("failed to send ntfy notification")
		}
	}
	return nil
}

func (e *Engine) RunResync(ctx context.Context, mappingID int64) error {
	if e.cfg.SonarrAPIKey == "" {
		return fmt.Errorf("sonarr API key not configured")
	}

	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil || mapping == nil {
		return fmt.Errorf("mapping not found")
	}

	series, err := e.sonarr.GetSeries(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return fmt.Errorf("fetching Sonarr series: %w", err)
	}

	result, err := e.processSeries(ctx, *series, mapping, 0, true)
	if err != nil {
		return fmt.Errorf("processing series: %w", err)
	}

	log.Info().Str("series", mapping.SonarrTitle).Int("episodes_updated", result.EpisodesUpdated).Msg("resync complete")
	return nil
}

func (e *Engine) InitialWatermarkForSeries(ctx context.Context, seriesID int) (int, error) {
	sonarrEpisodes, err := e.sonarr.GetEpisodes(ctx, seriesID)
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
	series, err := e.sonarr.GetSeries(ctx, seriesID)
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

	series, err := e.sonarr.GetSeries(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching Sonarr series: %w", err)
	}

	fillerTagID := 0
	if e.cfg.EnableTagging {
		fillerTagID, err = e.sonarr.EnsureTag(ctx, e.cfg.FillerTagName)
		if err != nil {
			log.Warn().Err(err).Msg("failed to ensure filler tag during mapping hydration")
		}
	}

	if _, err := e.processSeries(ctx, *series, mapping, fillerTagID, false); err != nil {
		return nil, err
	}

	return e.db.GetMappingByID(ctx, mappingID)
}

func (e *Engine) processSeries(ctx context.Context, series models.SonarrSeries, mapping *models.ShowMapping, fillerTagID int, resyncAll bool) (processSeriesResult, error) {
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

	sonarrEpisodes, err := e.sonarr.GetEpisodes(ctx, series.ID)
	if err != nil {
		return processSeriesResult{}, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}

	watermarkChanged, effectiveWatermark := e.refreshWatermark(ctx, mapping, sonarrEpisodes)
	if watermarkChanged {
		log.Info().Str("series", series.Title).Int("watermark", effectiveWatermark).Str("mode", string(mapping.WatermarkMode)).Msg("updated sync watermark")
	}

	var toUnmonitor []int
	var toMonitor []int
	dbEpisodes := make([]models.EpisodeInfo, 0, len(sonarrEpisodes))
	result := processSeriesResult{}

	for _, ep := range sonarrEpisodes {
		aflNum, aflEpisode, found := selectAFLEpisodeForSonarr(ep, aflByNumber, aflPreferred)
		if aflNum <= 0 {
			continue
		}
		if isSpecialSeason(ep.SeasonNumber) && !found {
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

		if !isSpecialSeason(ep.SeasonNumber) {
			dbEpisodes = append(dbEpisodes, models.EpisodeInfo{
				MappingID:  mapping.ID,
				EpisodeNum: aflNum,
				Title:      title,
				Type:       epType,
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

	if err := e.db.BulkUpsertEpisodes(ctx, dbEpisodes); err != nil {
		log.Warn().Err(err).Str("series", series.Title).Msg("failed to cache episode data")
	}

	if len(toUnmonitor) > 0 {
		if err := e.sonarr.BulkSetEpisodesMonitored(ctx, toUnmonitor, false); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("failed to unmonitor episodes")
		} else {
			result.UnmonitoredCount = len(toUnmonitor)
			result.EpisodesUpdated += len(toUnmonitor)
			log.Info().Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("unmonitored episodes")
		}
	}
	if len(toMonitor) > 0 {
		if err := e.sonarr.BulkSetEpisodesMonitored(ctx, toMonitor, true); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toMonitor)).Msg("failed to monitor episodes")
		} else {
			result.MonitoredCount = len(toMonitor)
			result.EpisodesUpdated += len(toMonitor)
			log.Info().Str("series", series.Title).Int("count", len(toMonitor)).Msg("monitored episodes")
		}
	}

	if e.cfg.EnableTagging && fillerTagID > 0 {
		hasFiller := false
		for _, ep := range aflEpisodes {
			if ep.Type == models.EpisodeFiller || ep.Type == models.EpisodeMixedCanon || ep.Type == models.EpisodeAnimeCanon {
				hasFiller = true
				break
			}
		}
		if hasFiller {
			if err := e.sonarr.AddTagToSeries(ctx, &series, fillerTagID); err != nil {
				log.Warn().Err(err).Str("series", series.Title).Msg("failed to add filler tag")
			}
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

func buildAFLEpisodeLookup(aflEpisodes []models.AFLEpisode) (map[int][]models.AFLEpisode, map[int]models.AFLEpisode) {
	return BuildAFLEpisodeLookup(aflEpisodes)
}

func selectAFLEpisodeForSonarr(sonarrEp models.SonarrEpisode, aflByNumber map[int][]models.AFLEpisode, aflPreferred map[int]models.AFLEpisode) (int, models.AFLEpisode, bool) {
	return SelectAFLEpisodeForSonarr(sonarrEp, aflByNumber, aflPreferred)
}

func matchSpecialAFLEpisodeByTitle(title string, candidates []models.AFLEpisode) (models.AFLEpisode, bool) {
	return matchAFLEpisodeByTitle(title, candidates)
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
		if ep.SeasonNumber == 0 {
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

	mapping := &models.ShowMapping{
		SonarrSeriesID:   series.ID,
		SonarrTitle:      series.Title,
		AFLSlug:          bestMatch.AFLShow.Slug,
		AFLTitle:         bestMatch.AFLShow.Title,
		AutoMatched:      true,
		MatchConfidence:  bestMatch.Confidence,
		FillerAction:     models.FillerAction(e.cfg.DefaultFillerAction),
		MixedAction:      models.FillerAction(e.cfg.DefaultMixedCanonAction),
		AnimeCanonAction: models.FillerAction(e.cfg.DefaultAnimeCanonAction),
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
