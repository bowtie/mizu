package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mizu/internal/config"
	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

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
	sonarrEpisodes, noAFLEntry := AlignToAFL(sonarrEpisodes, aflEpisodes)

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

	e.cfg.RLock()
	hold := config.HoldUnclassifiedDuration(e.cfg.HoldUnclassified)
	e.cfg.RUnlock()
	held, err := e.db.GetHeldEpisodes(ctx, mapping.ID)
	if err != nil {
		return processSeriesResult{}, fmt.Errorf("loading held episodes: %w", err)
	}
	now := time.Now()

	var toUnmonitor []int
	var toMonitor []int
	// toHold and toRelease update held_episodes once Sonarr has applied
	// the monitor changes they come with.
	var toHold, toRelease []int
	seen := make(map[int]bool, len(sonarrEpisodes))
	dbEpisodes := make([]models.EpisodeInfo, 0, len(sonarrEpisodes))
	result := processSeriesResult{}

	for _, ep := range sonarrEpisodes {
		seen[ep.ID] = true
		heldAt, isHeld := held[ep.ID]
		aflNum, aflEpisode, found := SelectAFLEpisodeForSonarr(ep, aflByNumber, aflPreferred)
		if noAFLEntry[ep.ID] {
			// An episode AFL doesn't count (a banned one, say): left
			// unclassified rather than given a neighbour's type.
			aflEpisode, found = models.AFLEpisode{}, false
		}
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

		// Specials are listed only when they match an AFL episode, like the
		// Clear Card prologue that AFL counts as episode 71.
		if !isSpecialSeason(ep.SeasonNumber) || found {
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

		// An episode AnimeFillerList hasn't listed yet may wait unmonitored
		// for it; once released (or never held) the rules below apply.
		if !found {
			switch holdFor(ep, heldAt, isHeld, hold, now) {
			case holdStart:
				toHold = append(toHold, ep.ID)
				if ep.Monitored {
					toUnmonitor = append(toUnmonitor, ep.ID)
				}
				continue
			case holdKeep:
				result.HeldCount++
				continue
			case holdRelease:
				toRelease = append(toRelease, ep.ID)
			}
		} else if isHeld {
			toRelease = append(toRelease, ep.ID)
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

	for id := range held {
		if !seen[id] {
			toRelease = append(toRelease, id)
		}
	}
	// A hold only counts once Sonarr unmonitored the episode, and a release
	// once it monitored it again; otherwise the next sync tries anew.
	if unmonitorSucceeded || len(toUnmonitor) == 0 {
		e.recordHeld(ctx, mapping.ID, toHold, true)
		result.HeldCount += len(toHold)
	}
	if monitorSucceeded || len(toMonitor) == 0 {
		e.recordHeld(ctx, mapping.ID, toRelease, false)
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
	if result.HeldCount > 0 {
		parts = append(parts, fmt.Sprintf("holding %d for AnimeFillerList", result.HeldCount))
	}
	if result.EpisodesUpdated == 0 {
		parts = append(parts, "no monitor changes")
	}
	return strings.Join(parts, " • ")
}
