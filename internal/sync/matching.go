package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

func (e *Engine) InitialWatermarkForSeries(ctx context.Context, seriesID int, aflSlug string) (int, error) {
	sonarrEpisodes, err := e.sonarrClient().GetEpisodes(ctx, seriesID)
	if err != nil {
		return 0, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	_, aflEpisodes, err := e.scraper.GetShowEpisodes(ctx, aflSlug)
	if err != nil {
		return 0, fmt.Errorf("fetching AFL episodes: %w", err)
	}
	return initialWatermarkForEpisodes(Place(sonarrEpisodes, aflEpisodes), time.Now()), nil
}

func (e *Engine) InitialWatermarkForStartMode(ctx context.Context, seriesID int, aflSlug string, startMode models.StartMode) (int, error) {
	if normalizeStartMode(startMode) == models.StartModeFromBeginning {
		return 0, nil
	}
	return e.InitialWatermarkForSeries(ctx, seriesID, aflSlug)
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

func (e *Engine) autoMatch(ctx context.Context, series models.SonarrSeries, bestMatch matcherpkg.MatchResult) (*models.ShowMapping, error) {
	log.Info().Str("sonarr", series.Title).Str("afl", bestMatch.AFLShow.Title).Float64("confidence", bestMatch.Confidence).Str("method", bestMatch.Method).Msg("auto-matched series")

	initialWatermark, err := e.InitialWatermarkForStartMode(ctx, series.ID, bestMatch.AFLShow.Slug, models.StartModeCurrentProgress)
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
