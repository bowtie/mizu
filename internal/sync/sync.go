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
