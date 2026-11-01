package artwork

import (
	"context"

	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

// RefreshResult is the per-show outcome of a RefreshAll operation.
type RefreshResult struct {
	MappingID int64  `json:"mapping_id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Found     bool   `json:"found"`
	Error     string `json:"error,omitempty"`
}

// RefreshAllSummary is the aggregate result returned by RefreshAll.
type RefreshAllSummary struct {
	Total     int             `json:"total"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	NotFound  int             `json:"not_found"`
	Results   []RefreshResult `json:"results"`
}

// RefreshAll iterates over every mapping and re-downloads artwork from the
// resolved source (per-show override > global default). It fetches series
// metadata from Sonarr when available. The seriesFetcher callback lets the
// caller decide how to obtain Sonarr series data (synced or live).
func (m *Manager) RefreshAll(ctx context.Context, mappings []models.ShowMapping, defaultSource string, seriesFetcher func(ctx context.Context, sonarrSeriesID int) (*models.SonarrSeries, error)) RefreshAllSummary {
	summary := RefreshAllSummary{Total: len(mappings), Results: make([]RefreshResult, 0, len(mappings))}

	// Capture each mapping's intent at the snapshot moment: a per-show
	// refresh or source switch that lands during the run bumps it, and the
	// per-show fetch for that mapping then skips its write-back instead of
	// clobbering the newer choice (see fetchAndStore). Capturing here — not
	// per mapping as the loop reaches it — is what makes the refresh-all the
	// older intent for every mapping it touches.
	snapshot := make(map[int64]int64, len(mappings))
	for _, mapping := range mappings {
		snapshot[mapping.ID] = m.captureIntent(mapping.ID)
	}

	for _, mapping := range mappings {
		select {
		case <-ctx.Done():
			return summary
		default:
		}

		result := RefreshResult{MappingID: mapping.ID, Title: mapping.SonarrTitle}
		source := m.ResolveSource(mapping.ArtworkSource, defaultSource)
		result.Source = source

		series := models.SonarrSeries{
			ID:    mapping.SonarrSeriesID,
			Title: mapping.SonarrTitle,
		}
		if seriesFetcher != nil {
			if fetched, err := seriesFetcher(ctx, mapping.SonarrSeriesID); err == nil && fetched != nil {
				series = *fetched
			} else if err != nil {
				log.Warn().Err(err).Int("sonarr_series_id", mapping.SonarrSeriesID).Msg("failed to fetch Sonarr series for artwork refresh")
			}
		}
		// AFL slug is a clean, normalized title — add it as an alternate title
		// so providers can use it for matching when the Sonarr title is corrupted.
		if mapping.AFLSlug != "" {
			series.AlternateTitles = append(series.AlternateTitles, models.SonarrAltTitle{Title: mapping.AFLSlug})
		}

		fr, err := m.fetchAndStore(ctx, mapping.ID, series, source, snapshot[mapping.ID])
		if err != nil {
			result.Error = err.Error()
			summary.Failed++
		} else if fr != nil && fr.Found {
			result.Found = true
			summary.Succeeded++
		} else {
			summary.NotFound++
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}
