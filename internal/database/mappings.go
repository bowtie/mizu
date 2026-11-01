package database

import (
	"context"
	"database/sql"
	"time"

	"mizu/internal/database/sqlcgen"
	"mizu/internal/models"

	_ "modernc.org/sqlite"
)

func (db *DB) GetAllMappings(ctx context.Context) ([]models.ShowMapping, error) {
	rows, err := db.q.GetAllMappings(ctx)
	if err != nil {
		return nil, err
	}
	mappings := make([]models.ShowMapping, 0, len(rows))
	for _, r := range rows {
		mappings = append(mappings, normalizeMapping(sqlcMappingToModel(r)))
	}
	return mappings, nil
}

func (db *DB) GetMappingByID(ctx context.Context, id int64) (*models.ShowMapping, error) {
	row, err := db.q.GetMappingByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := normalizeMapping(sqlcMappingToModel(row))
	return &m, nil
}

func (db *DB) GetMappingBySonarrID(ctx context.Context, seriesID int) (*models.ShowMapping, error) {
	row, err := db.q.GetMappingBySonarrID(ctx, int64(seriesID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := normalizeMapping(sqlcMappingToModel(row))
	return &m, nil
}

func (db *DB) UpsertMapping(ctx context.Context, m *models.ShowMapping) error {
	if m.StartMode == "" {
		m.StartMode = models.StartModeCurrentProgress
	}
	if m.WatermarkMode == "" {
		if m.StartMode == models.StartModeFromBeginning {
			m.WatermarkMode = models.WatermarkModeManual
		} else {
			m.WatermarkMode = models.WatermarkModeAuto
		}
	}
	if m.SyncWatermark < 0 {
		m.SyncWatermark = 0
	}
	*m = normalizeMapping(*m)
	now := time.Now()
	m.UpdatedAt = now

	err := db.q.UpsertMapping(ctx, sqlcgen.UpsertMappingParams{
		SonarrSeriesID:   int64(m.SonarrSeriesID),
		SonarrTitle:      m.SonarrTitle,
		AflSlug:          m.AFLSlug,
		AflTitle:         m.AFLTitle,
		AutoMatched:      boolToInt(m.AutoMatched),
		MatchConfidence:  m.MatchConfidence,
		FillerAction:     string(m.FillerAction),
		MixedAction:      string(m.MixedAction),
		AnimeCanonAction: string(m.AnimeCanonAction),
		Enabled:          boolToInt(m.Enabled),
		StartMode:        string(m.StartMode),
		WatermarkMode:    string(m.WatermarkMode),
		SyncWatermark:    int64(m.SyncWatermark),
		Overview:         m.Overview,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		return err
	}

	// Populate the ID on insert (autoincrement). On conflict (update),
	// LastInsertId returns 0 in modernc.org/sqlite, so fetch it if needed.
	if m.ID == 0 {
		id, err := db.q.GetMappingIDBySonarrID(ctx, int64(m.SonarrSeriesID))
		if err == nil {
			m.ID = id
		}
	}
	return nil
}

func (db *DB) UpdateArtwork(ctx context.Context, mappingID int64, posterPath, bannerPath, logoPath, posterURL, bannerURL, logoURL string) error {
	return db.q.UpdateArtwork(ctx, sqlcgen.UpdateArtworkParams{
		ArtworkPosterPath: posterPath,
		ArtworkBannerPath: bannerPath,
		ArtworkLogoPath:   logoPath,
		ArtworkPosterUrl:  posterURL,
		ArtworkBannerUrl:  bannerURL,
		ArtworkLogoUrl:    logoURL,
		ID:                mappingID,
	})
}

func (db *DB) UpdateArtworkSource(ctx context.Context, mappingID int64, source string) error {
	return db.q.UpdateArtworkSource(ctx, sqlcgen.UpdateArtworkSourceParams{
		ArtworkSource: source,
		ID:            mappingID,
	})
}

func (db *DB) UpdateOverview(ctx context.Context, mappingID int64, overview string) error {
	return db.q.UpdateOverview(ctx, sqlcgen.UpdateOverviewParams{
		Overview: overview,
		ID:       mappingID,
	})
}

func (db *DB) UpdateWatermarkState(ctx context.Context, mappingID int64, watermark int, mode models.WatermarkMode) error {
	if watermark < 0 {
		watermark = 0
	}
	if mode == "" {
		mode = models.WatermarkModeAuto
	}
	return db.q.UpdateWatermarkState(ctx, sqlcgen.UpdateWatermarkStateParams{
		SyncWatermark: int64(watermark),
		WatermarkMode: string(mode),
		ID:            mappingID,
	})
}

func (db *DB) DeleteMapping(ctx context.Context, id int64) error {
	return db.q.DeleteMapping(ctx, id)
}

// ── Episodes ──
