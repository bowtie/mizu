package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mizu/internal/models"
	"mizu/internal/sonarr"

	"github.com/rs/zerolog/log"
)

// runPollInterval is how often a webhook resync retries the run slot while
// another sync holds it.
const runPollInterval = 2 * time.Second

// queuePollDelays space out the download queue lookups after a grab:
// Sonarr lists a download only once it has polled the download client,
// which can take a minute or two after the grab itself.
var queuePollDelays = []time.Duration{0, 10 * time.Second, 20 * time.Second, 30 * time.Second, time.Minute}

// RunWebhook resyncs a mapping after a Sonarr event, waiting for the run
// slot if a sync holds it, and records the run in the activity log.
//
// For a grab (grabbed episode IDs and the download ID), it then cancels the
// download if every grabbed episode was monitored and this resync
// unmonitored it: Sonarr grabbed a filler before Mizu caught up. A grab of
// an episode that was already unmonitored was made by hand and is left
// alone.
func (e *Engine) RunWebhook(ctx context.Context, mappingID int64, grabbed []int, downloadID string) error {
	e.cfg.RLock()
	apiKey := e.cfg.SonarrAPIKey
	e.cfg.RUnlock()
	if apiKey == "" {
		return fmt.Errorf("sonarr API key not configured")
	}

	done, err := e.waitForRun(ctx)
	if err != nil {
		return fmt.Errorf("waiting for the running sync: %w", err)
	}
	// Same ordering as RunResyncClaimed: capture the epoch before reading.
	epochs := map[int64]int64{mappingID: e.CaptureMappingEpoch(mappingID)}
	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil || mapping == nil {
		done()
		return fmt.Errorf("mapping not found")
	}

	logID, lerr := e.db.CreateSyncLog(ctx, "webhook")
	if lerr != nil {
		log.Error().Err(lerr).Msg("failed to create sync log")
	}
	item := models.SyncLogItem{MappingID: mapping.ID, SonarrTitle: mapping.SonarrTitle}

	result, cancel, err := e.webhookResync(ctx, mapping, epochs, grabbed)
	done()
	switch {
	case errors.Is(err, errModifiedDuringSync):
		item.Status = "skipped"
		item.Summary = "Skipped: modified during sync"
		err = nil
	case err != nil:
		item.Status = "failed"
		item.Summary = "Failed before sync could finish"
		item.Error = err.Error()
	default:
		item.Status = "success"
		item.EpisodesChanged = result.EpisodesUpdated
		item.Summary = summarizeProcessResult(result)
		if cancel && downloadID != "" {
			// Outside the run slot: finding the download can take minutes.
			cancelled, cerr := e.cancelDownload(ctx, mapping.SonarrSeriesID, downloadID)
			switch {
			case cerr != nil:
				item.Status = "failed"
				item.Error = cerr.Error()
				item.Summary += " • couldn't cancel the download"
				err = cerr
			case cancelled:
				item.Summary += " • cancelled the download"
				log.Info().Str("series", mapping.SonarrTitle).Msg("cancelled a filler download")
			default:
				item.Summary += " • download not found in queue"
			}
		}
	}

	processed, errCount := 0, 0
	switch item.Status {
	case "success":
		processed = 1
	case "failed":
		errCount = 1
	}
	if lerr := e.db.ReplaceSyncLogItems(ctx, logID, []models.SyncLogItem{item}); lerr != nil {
		log.Error().Err(lerr).Int64("log_id", logID).Msg("failed to store sync log items")
	}
	e.completeSyncLog(ctx, logID, runStatus(errCount, processed), processed, item.EpisodesChanged, item.Error)
	return err
}

// waitForRun claims the run slot, retrying while another sync holds it.
func (e *Engine) waitForRun(ctx context.Context) (func(), error) {
	ticker := time.NewTicker(runPollInterval)
	defer ticker.Stop()
	for {
		if done, ok := e.TryAcquireRun(); ok {
			return done, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// webhookResync processes the mapping's series and reports whether a grab
// of the given episodes should be cancelled.
func (e *Engine) webhookResync(ctx context.Context, mapping *models.ShowMapping, epochs map[int64]int64, grabbed []int) (processSeriesResult, bool, error) {
	sc := e.sonarrClient()
	series, err := sc.GetSeries(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return processSeriesResult{}, false, fmt.Errorf("fetching Sonarr series: %w", err)
	}

	var before map[int]bool
	if len(grabbed) > 0 {
		eps, err := sc.GetEpisodes(ctx, series.ID)
		if err != nil {
			return processSeriesResult{}, false, fmt.Errorf("fetching Sonarr episodes: %w", err)
		}
		before = monitoredByID(eps)
	}

	result, err := e.processSeries(ctx, *series, mapping, false, epochs)
	if err != nil || before == nil {
		return result, false, err
	}

	eps, err := sc.GetEpisodes(ctx, series.ID)
	if err != nil {
		return result, false, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	return result, newlyUnmonitored(grabbed, before, monitoredByID(eps)), nil
}

// cancelDownload removes every queue entry of downloadID, polling the queue
// until the download shows up. It reports whether anything was removed.
func (e *Engine) cancelDownload(ctx context.Context, seriesID int, downloadID string) (bool, error) {
	sc := e.sonarrClient()
	for _, delay := range queuePollDelays {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(delay):
		}
		queue, err := sc.GetQueueForSeries(ctx, seriesID)
		if err != nil {
			return false, fmt.Errorf("reading the Sonarr queue: %w", err)
		}
		matches := queueEntriesFor(queue, downloadID)
		if len(matches) == 0 {
			continue
		}
		for _, id := range matches {
			if err := sc.RemoveQueueItem(ctx, id); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

func monitoredByID(eps []models.SonarrEpisode) map[int]bool {
	m := make(map[int]bool, len(eps))
	for _, ep := range eps {
		m[ep.ID] = ep.Monitored
	}
	return m
}

// newlyUnmonitored reports whether every grabbed episode went from
// monitored to unmonitored.
func newlyUnmonitored(grabbed []int, before, after map[int]bool) bool {
	if len(grabbed) == 0 {
		return false
	}
	for _, id := range grabbed {
		if !before[id] || after[id] {
			return false
		}
	}
	return true
}

// queueEntriesFor returns the IDs of the queue entries belonging to
// downloadID. Download IDs are torrent hashes or client IDs, compared
// without regard to case.
func queueEntriesFor(queue []sonarr.QueueItem, downloadID string) []int {
	var ids []int
	for _, q := range queue {
		if strings.EqualFold(q.DownloadID, downloadID) {
			ids = append(ids, q.ID)
		}
	}
	return ids
}
