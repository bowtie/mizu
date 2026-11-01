package database

import (
	"context"
	"time"

	"mizu/internal/database/sqlcgen"
	"mizu/internal/models"

	_ "modernc.org/sqlite"
)

func (db *DB) GetEpisodes(ctx context.Context, mappingID int64) ([]models.EpisodeInfo, error) {
	rows, err := db.q.GetEpisodesByMapping(ctx, mappingID)
	if err != nil {
		return nil, err
	}
	episodes := make([]models.EpisodeInfo, 0, len(rows))
	for _, r := range rows {
		episodes = append(episodes, sqlcEpisodeToModel(r))
	}
	return episodes, nil
}

func (db *DB) UpdateEpisodeMonitor(ctx context.Context, mappingID, sonarrEpisodeID int64, monitored bool) error {
	return db.q.UpdateEpisodeMonitor(ctx, sqlcgen.UpdateEpisodeMonitorParams{
		Monitored:       boolToInt(monitored),
		MappingID:       mappingID,
		SonarrEpisodeID: sonarrEpisodeID,
	})
}

func (db *DB) ReplaceEpisodesForMapping(ctx context.Context, mappingID int64, episodes []models.EpisodeInfo) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := db.q.WithTx(tx)

	if err := qtx.DeleteEpisodesByMapping(ctx, mappingID); err != nil {
		return err
	}

	if len(episodes) == 0 {
		return tx.Commit()
	}

	now := time.Now()
	for _, e := range episodes {
		var absNum *int64
		if e.AbsoluteEpisodeNumber != nil {
			v := int64(*e.AbsoluteEpisodeNumber)
			absNum = &v
		}
		var airDate *time.Time
		if e.AirDate != nil {
			t := time.Time(*e.AirDate)
			airDate = &t
		}
		if err := qtx.InsertEpisode(ctx, sqlcgen.InsertEpisodeParams{
			MappingID:             mappingID,
			EpisodeNum:            int64(e.EpisodeNum),
			Title:                 e.Title,
			Type:                  string(e.Type),
			LastScraped:           now,
			SonarrEpisodeID:       int64(e.SonarrEpisodeID),
			SeasonNumber:          int64(e.SeasonNumber),
			SonarrEpisodeNumber:   int64(e.SonarrEpisodeNumber),
			AbsoluteEpisodeNumber: absNum,
			Monitored:             boolToInt(e.Monitored),
			AirDate:               airDate,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ── Sync logs ──
