package sync

import (
	"context"
	"time"

	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

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

func (e *Engine) refreshWatermark(ctx context.Context, mapping *models.ShowMapping, episodes []PlacedEpisode) (bool, int) {
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

func nextAutoWatermark(current int, episodes []PlacedEpisode, now time.Time) int {
	computed := initialWatermarkForEpisodes(episodes, now)
	if computed < current {
		return current
	}
	return computed
}

// initialWatermarkForEpisodes is the highest AFL-numbered episode Sonarr has
// downloaded and that has aired.
func initialWatermarkForEpisodes(episodes []PlacedEpisode, now time.Time) int {
	maxWatermark := 0
	for _, p := range episodes {
		ep := p.Sonarr
		// An episode AFL doesn't count has no AFL number to reach.
		if isSpecialSeason(ep.SeasonNumber) || p.Kind == NotOnAFL {
			continue
		}
		if ep.AirDateUTC.After(now) {
			continue
		}
		if !ep.HasFile {
			continue
		}
		// Same guard as selectAFLEpisode: without an absolute
		// episode number, aflNumberOf falls back to the season-relative
		// number, which would collide with early AFL entries (S23E17 → 17)
		// and compute a watermark far below actual progress. Skip instead —
		// the watermark then only advances on episodes with real absolute
		// numbers.
		if p.Absolute == nil || *p.Absolute <= 0 {
			continue
		}
		aflNum := *p.Absolute
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
