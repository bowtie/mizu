package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mizu/internal/config"
	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/metrics"
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

	// epochMu guards mappingEpochs. A per-mapping epoch is bumped by
	// user-facing writes (monitor toggles, mapping edits) so an in-flight
	// sync can detect that its snapshot predates the edit and skip its
	// stale write-backs for that mapping instead of reverting the edit.
	epochMu       sync.Mutex
	mappingEpochs map[int64]int64
}

// errModifiedDuringSync is returned by processSeries when a user write for
// the mapping landed after the sync snapshotted it, so the sync's stale
// write-backs are skipped rather than reverting the user's edit.
var errModifiedDuringSync = errors.New("mapping modified during sync")

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
		cfg:           cfg,
		db:            db,
		sonarr:        sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, cfg.RequestTimeout),
		scraper:       scraper.New(cfg.RequestTimeout, cfg.RateLimitDelay, db),
		matcher:       matcherpkg.New(0.82),
		mappingEpochs: make(map[int64]int64),
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
func (e *Engine) SetRunning(v bool) { e.running.Store(v) }

// BumpMappingEpoch records a user-initiated change to a mapping. Called by
// handlers that persist a user write (episode monitor toggles, mapping edits)
// so an in-flight sync — which snapshotted the mapping before the edit — skips
// its stale write-backs for it instead of reverting the edit.
func (e *Engine) BumpMappingEpoch(mappingID int64) {
	e.epochMu.Lock()
	e.mappingEpochs[mappingID]++
	e.epochMu.Unlock()
}

// CaptureMappingEpoch returns the mapping's current epoch. The sync captures
// it when it snapshots the mapping, then compares before any write-back.
func (e *Engine) CaptureMappingEpoch(mappingID int64) int64 {
	e.epochMu.Lock()
	defer e.epochMu.Unlock()
	return e.mappingEpochs[mappingID]
}

// mappingModifiedSinceSnapshot reports whether a user write landed for the
// mapping after the sync captured its epoch. epochs is nil for callers that
// don't participate in the guard.
func (e *Engine) mappingModifiedSinceSnapshot(mappingID int64, epochs map[int64]int64) bool {
	captured, ok := epochs[mappingID]
	return ok && e.CaptureMappingEpoch(mappingID) != captured
}

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

// TryAcquireRun atomically claims the engine's single-run slot without
// starting work. It closes the check-then-act gap in HTTP handlers:
// checking IsRunning and then spawning a goroutine that CASes again
// could return 202 for a sync that instantly no-ops. Callers that claim
// the slot MUST eventually call done — normally deferred in the spawned
// goroutine around a RunSyncClaimed/RunResyncClaimed call.
func (e *Engine) TryAcquireRun() (done func(), ok bool) {
	if !e.running.CompareAndSwap(false, true) {
		return func() {}, false
	}
	var once sync.Once
	return func() { once.Do(func() { e.running.Store(false) }) }, true
}

func (e *Engine) RunSync(ctx context.Context, trigger string) error {
	done, ok := e.TryAcquireRun()
	if !ok {
		return fmt.Errorf("sync already in progress")
	}
	defer done()
	return e.runSyncClaimed(ctx, trigger)
}

// RunSyncClaimed runs a sync while already holding the claim from
// TryAcquireRun; it does not release the slot — the caller's done func
// stays responsible for that.
func (e *Engine) RunSyncClaimed(ctx context.Context, trigger string) error {
	return e.runSyncClaimed(ctx, trigger)
}

func (e *Engine) runSyncClaimed(ctx context.Context, trigger string) error {
	e.cfg.RLock()
	apiKey := e.cfg.SonarrAPIKey
	e.cfg.RUnlock()
	if apiKey == "" {
		e.running.Store(false)
		log.Warn().Msg("skipping sync: no Sonarr API key configured")
		return fmt.Errorf("sonarr API key not configured")
	}

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

	// Capture the per-mapping epoch at snapshot time: any user write that
	// lands later in the run bumps it, and processSeries then skips its
	// stale write-backs for that mapping (see errModifiedDuringSync).
	epochs := make(map[int64]int64, len(enabledMappings))
	for _, m := range enabledMappings {
		epochs[m.ID] = e.CaptureMappingEpoch(m.ID)
	}

	// Process mappings concurrently with a small worker pool. The scraper's
	// rate limiter ensures AFL requests stay spaced out, while Sonarr API
	// calls overlap — cutting wall-clock time roughly 3x for large libraries.
	results := e.processMappingsConcurrent(ctx, enabledMappings, epochs)

	showsProcessed := 0
	episodesUpdated := 0
	var syncErrors []string
	var totalFiller, totalMixedCanon, totalAnimeCanon int
	runItems := make([]models.SyncLogItem, 0, len(enabledMappings))
	for _, res := range results {
		if res.skipped {
			log.Info().Str("series", res.mapping.SonarrTitle).Msg("sync skipped: mapping modified during run")
			runItems = append(runItems, models.SyncLogItem{
				MappingID:   res.mapping.ID,
				SonarrTitle: res.mapping.SonarrTitle,
				Status:      "skipped",
				Summary:     "Skipped — modified during sync",
			})
			continue
		}
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

	e.cfg.RLock()
	notifyOn := e.cfg.NtfyNotifyOn
	e.cfg.RUnlock()
	if n := e.notifierClient(); n != nil && n.Enabled() && shouldNotify(notifyOn, status, episodesUpdated) {
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
	skipped  bool // user wrote to the mapping mid-run; write-backs were skipped
}

// processMappingsConcurrent runs mappings through a worker pool. The worker
// count is capped at 3 — enough to overlap Sonarr API calls (which are the
// main latency source) while keeping AFL scraper concurrency at 1 (the rate
// limiter serializes those anyway). Results are returned in input order so
// sync log items are deterministic.
func (e *Engine) processMappingsConcurrent(ctx context.Context, mappings []models.ShowMapping, epochs map[int64]int64) []mappingResult {
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
			procResult, err := e.processSeries(ctx, *series, &mappings[idx], false, epochs)
			if errors.Is(err, errModifiedDuringSync) {
				res.skipped = true
				results[idx] = res
				continue
			}
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
	done, ok := e.TryAcquireRun()
	if !ok {
		return fmt.Errorf("sync already in progress")
	}
	defer done()
	return e.RunResyncClaimed(ctx, mappingID)
}

// RunResyncClaimed runs a resync while already holding the claim from
// TryAcquireRun; it does not release the slot.
func (e *Engine) RunResyncClaimed(ctx context.Context, mappingID int64) error {
	e.cfg.RLock()
	apiKey := e.cfg.SonarrAPIKey
	e.cfg.RUnlock()
	if apiKey == "" {
		return fmt.Errorf("sonarr API key not configured")
	}

	// Capture the epoch before reading so a write landing after this point
	// bumps it and is detected by processSeries's guards — a write between
	// the read and the capture would otherwise be absorbed into the snapshot.
	epochs := map[int64]int64{mappingID: e.CaptureMappingEpoch(mappingID)}

	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil || mapping == nil {
		return fmt.Errorf("mapping not found")
	}

	series, err := e.sonarrClient().GetSeries(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return fmt.Errorf("fetching Sonarr series: %w", err)
	}

	result, err := e.processSeries(ctx, *series, mapping, true, epochs)
	if errors.Is(err, errModifiedDuringSync) {
		log.Info().Str("series", mapping.SonarrTitle).Msg("resync skipped: mapping modified during run")
		return nil
	}
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
	// Capture the epoch before reading so a write landing after this point
	// bumps it and is detected by processSeries's guards — a write between
	// the read and the capture would otherwise be absorbed into the snapshot.
	// The caller's own edit (if any) bumped before HydrateMapping was called,
	// so this hydrate's own write-backs are still not skipped.
	epochs := map[int64]int64{mappingID: e.CaptureMappingEpoch(mappingID)}

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

	if _, err := e.processSeries(ctx, *series, mapping, false, epochs); err != nil {
		if errors.Is(err, errModifiedDuringSync) {
			// A newer user write superseded this hydrate; return the mapping
			// as-is — the user's own handler re-hydrates with fresh state.
			return e.db.GetMappingByID(ctx, mappingID)
		}
		return nil, err
	}

	return e.db.GetMappingByID(ctx, mappingID)
}

func (e *Engine) processSeries(ctx context.Context, series models.SonarrSeries, mapping *models.ShowMapping, resyncAll bool, epochs map[int64]int64) (processSeriesResult, error) {
	mapping.StartMode = normalizeStartMode(mapping.StartMode)
	mapping.WatermarkMode = normalizeWatermarkMode(mapping.WatermarkMode, mapping.StartMode)

	// Supersede guard (early): a user write may have landed after the sync
	// snapshotted this mapping — skip the expensive fetches entirely.
	if e.mappingModifiedSinceSnapshot(mapping.ID, epochs) {
		return processSeriesResult{}, errModifiedDuringSync
	}

	aflTitle, aflEpisodes, err := e.scraper.GetShowEpisodes(ctx, mapping.AFLSlug)
	if err != nil {
		return processSeriesResult{}, fmt.Errorf("fetching AFL episodes: %w", err)
	}

	// Supersede guard (late): a user write may have landed while the fetches
	// were in flight. Check again right before the first write-back.
	if e.mappingModifiedSinceSnapshot(mapping.ID, epochs) {
		return processSeriesResult{}, errModifiedDuringSync
	}

	if mapping.AFLTitle == "" && aflTitle != "" {
		mapping.AFLTitle = aflTitle
		if uerr := e.db.UpsertMapping(ctx, mapping); uerr != nil {
			log.Warn().Err(uerr).Msg("failed to backfill AFL title")
		}
	}

	// Backfill overview if missing (older mappings created before the column existed)
	if mapping.Overview == "" && series.Overview != "" {
		mapping.Overview = series.Overview
		if uerr := e.db.UpdateOverview(ctx, mapping.ID, series.Overview); uerr != nil {
			log.Warn().Err(uerr).Msg("failed to backfill overview")
		}
	}

	aflByNumber, aflPreferred := BuildAFLEpisodeLookup(aflEpisodes)
	coverageMax := AFLCoverageMaxEpisodeNumber(aflEpisodes)

	sonarrEpisodes, err := e.sonarrClient().GetEpisodes(ctx, series.ID)
	if err != nil {
		return processSeriesResult{}, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}

	// Supersede guard (post-fetch): a user write may have landed while the
	// Sonarr episode fetch was in flight. Check again before the watermark
	// and episode write-backs so a stale auto-watermark or monitor list
	// computed from the snapshot can't clobber it.
	if e.mappingModifiedSinceSnapshot(mapping.ID, epochs) {
		return processSeriesResult{}, errModifiedDuringSync
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

		// For mainline episodes beyond AFL coverage without an absolute
		// episode number from Sonarr, compute one from the last known
		// absolute number in the same season so the episode displays
		// correctly (e.g. S23E17 → 1172, not 17).
		if !found && !isSpecialSeason(ep.SeasonNumber) && ep.AbsoluteEpisodeNumber == nil {
			lastAbs := 0
			lastSonarrEp := 0
			for _, dbe := range dbEpisodes {
				if dbe.SeasonNumber == ep.SeasonNumber && dbe.AbsoluteEpisodeNumber != nil {
					if *dbe.AbsoluteEpisodeNumber > lastAbs {
						lastAbs = *dbe.AbsoluteEpisodeNumber
						lastSonarrEp = dbe.SonarrEpisodeNumber
					}
				}
			}
			if lastAbs > 0 && lastSonarrEp > 0 {
				computed := lastAbs + (ep.EpisodeNumber - lastSonarrEp)
				// Only use the computed absolute number if it's greater than
				// the last known absolute — out-of-order episodes (recaps,
				// rebroadcasts) can produce incorrect or negative values.
				if computed > lastAbs {
					aflNum = computed
				}
			}
		}

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
			airDate := ep.AirDateUTC
			airDateOnly := models.DateOnly(airDate)
			dbEpisodes = append(dbEpisodes, models.EpisodeInfo{
				MappingID:             mapping.ID,
				EpisodeNum:            aflNum,
				Title:                 title,
				Type:                  epType,
				SonarrEpisodeID:       ep.ID,
				SeasonNumber:          ep.SeasonNumber,
				SonarrEpisodeNumber:   ep.EpisodeNumber,
				AbsoluteEpisodeNumber: ep.AbsoluteEpisodeNumber,
				AirDate:               &airDateOnly,
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

	// Build a set of episodes whose monitored status will change so
	// we can update the DB accordingly before saving.
	unmonitorSet := make(map[int]bool, len(toUnmonitor))
	for _, id := range toUnmonitor {
		unmonitorSet[id] = true
	}
	monitorSet := make(map[int]bool, len(toMonitor))
	for _, id := range toMonitor {
		monitorSet[id] = true
	}
	// Update dbEpisodes to reflect the post-sync monitored status so
	// the frontend sees the correct state immediately.
	for i := range dbEpisodes {
		if unmonitorSet[dbEpisodes[i].SonarrEpisodeID] {
			dbEpisodes[i].Monitored = false
		} else if monitorSet[dbEpisodes[i].SonarrEpisodeID] {
			dbEpisodes[i].Monitored = true
		}
	}

	// Supersede guard (pre-write): a user write may have landed during the
	// watermark refresh or the episode loop — skip the episode-table rewrite
	// and the Sonarr bulk apply so the newer state is never reverted.
	if e.mappingModifiedSinceSnapshot(mapping.ID, epochs) {
		return processSeriesResult{}, errModifiedDuringSync
	}

	if err := e.db.ReplaceEpisodesForMapping(ctx, mapping.ID, dbEpisodes); err != nil {
		log.Warn().Err(err).Str("series", series.Title).Msg("failed to cache episode data")
	}

	// Apply Sonarr bulk monitor changes first. Only update the DB monitored
	// status for episodes where the Sonarr operation succeeded, so the DB
	// stays in sync with Sonarr's actual state.
	unmonitorSucceeded := false
	if len(toUnmonitor) > 0 {
		if err := e.sonarrClient().BulkSetEpisodesMonitored(ctx, toUnmonitor, false); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("failed to unmonitor episodes")
		} else {
			unmonitorSucceeded = true
			result.UnmonitoredCount = len(toUnmonitor)
			result.EpisodesUpdated += len(toUnmonitor)
			log.Info().Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("unmonitored episodes")
		}
	}
	monitorSucceeded := false
	if len(toMonitor) > 0 {
		if err := e.sonarrClient().BulkSetEpisodesMonitored(ctx, toMonitor, true); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toMonitor)).Msg("failed to monitor episodes")
		} else {
			monitorSucceeded = true
			result.MonitoredCount = len(toMonitor)
			result.EpisodesUpdated += len(toMonitor)
			log.Info().Str("series", series.Title).Int("count", len(toMonitor)).Msg("monitored episodes")
		}
	}

	// Reconcile DB monitored status with Sonarr results. If a bulk operation
	// failed, revert the DB monitored status for those episodes to match
	// Sonarr's actual state.
	if len(toUnmonitor) > 0 && !unmonitorSucceeded {
		for i := range dbEpisodes {
			if unmonitorSet[dbEpisodes[i].SonarrEpisodeID] {
				dbEpisodes[i].Monitored = true
			}
		}
	}
	if len(toMonitor) > 0 && !monitorSucceeded {
		for i := range dbEpisodes {
			if monitorSet[dbEpisodes[i].SonarrEpisodeID] {
				dbEpisodes[i].Monitored = false
			}
		}
	}
	// If either bulk operation failed, re-persist the corrected DB state.
	if (len(toUnmonitor) > 0 && !unmonitorSucceeded) || (len(toMonitor) > 0 && !monitorSucceeded) {
		if err := e.db.ReplaceEpisodesForMapping(ctx, mapping.ID, dbEpisodes); err != nil {
			log.Warn().Err(err).Str("series", series.Title).Msg("failed to reconcile episode data after partial Sonarr failure")
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
	// Mainline episodes within AFL coverage are always included.
	// Episodes beyond coverageMax are also included but will be typed
	// as "unknown" (no AFL match) — they show up in the UI so the user
	// knows they exist, and get reclassified when AFL updates.
	return true
}

func SelectAFLEpisodeForSonarr(sonarrEp models.SonarrEpisode, aflByNumber map[int][]models.AFLEpisode, aflPreferred map[int]models.AFLEpisode) (int, models.AFLEpisode, bool) {
	aflNum := matcherpkg.MapEpisodeNumber(sonarrEp, 0)
	// For mainline episodes without an absolute episode number, the
	// season-relative number can collide with existing AFL entries (e.g.
	// S23E17 → aflNum 17, which matches AFL ep 17 from season 2). When
	// the absolute number is missing, skip AFL matching entirely — the
	// episode will be typed as "unknown" and get a computed absolute
	// number in the sync loop.
	if aflNum > 0 && !isSpecialSeason(sonarrEp.SeasonNumber) && sonarrEp.AbsoluteEpisodeNumber == nil {
		return aflNum, models.AFLEpisode{}, false
	}
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
		// Same guard as SelectAFLEpisodeForSonarr: without an absolute
		// episode number, MapEpisodeNumber falls back to the season-relative
		// number, which would collide with early AFL entries (S23E17 → 17)
		// and compute a watermark far below actual progress. Skip instead —
		// the watermark then only advances on episodes with real absolute
		// numbers.
		if ep.AbsoluteEpisodeNumber == nil || *ep.AbsoluteEpisodeNumber <= 0 {
			continue
		}
		aflNum := *ep.AbsoluteEpisodeNumber
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
	metrics.Add("mizu_sync_cycles_total", status)
	if episodesUpdated > 0 {
		metrics.AddN("mizu_sync_episodes_updated_total", uint64(episodesUpdated))
	}
	if status == "success" || status == "partial" {
		metrics.SetGauge("mizu_sync_last_success_timestamp_seconds", "", float64(time.Now().Unix()))
	}
	if logID == 0 {
		return
	}
	if err := e.db.CompleteSyncLog(ctx, logID, status, showsProcessed, episodesUpdated, errors); err != nil {
		log.Error().Err(err).Msg("failed to complete sync log")
	}
}

// shouldNotify reports whether a sync result should trigger an ntfy
// notification under the configured filter: "always" sends every run,
// "changes" sends when episodes changed or the run failed, and
// "failures" sends only for failed/partial runs.
func shouldNotify(mode, status string, episodesUpdated int) bool {
	switch mode {
	case "failures":
		return status == "failed" || status == "partial"
	case "changes":
		return episodesUpdated > 0 || status == "failed" || status == "partial"
	default:
		return true
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
		Overview:         series.Overview,
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
