package database

import (
	"context"
	"time"

	"mizu/internal/database/sqlcgen"
)

// GetHeldEpisodes returns when each held episode of a mapping was held.
func (db *DB) GetHeldEpisodes(ctx context.Context, mappingID int64) (map[int]time.Time, error) {
	rows, err := db.q.GetHeldEpisodes(ctx, mappingID)
	if err != nil {
		return nil, err
	}
	held := make(map[int]time.Time, len(rows))
	for _, r := range rows {
		held[int(r.SonarrEpisodeID)] = r.HeldAt
	}
	return held, nil
}

// HoldEpisodes records episodes as held from now; ones already held keep
// their original time.
func (db *DB) HoldEpisodes(ctx context.Context, mappingID int64, episodeIDs []int) error {
	return db.eachHeld(ctx, episodeIDs, func(q *sqlcgen.Queries, id int) error {
		return q.InsertHeldEpisode(ctx, sqlcgen.InsertHeldEpisodeParams{MappingID: mappingID, SonarrEpisodeID: int64(id), HeldAt: time.Now().UTC()})
	})
}

// ReleaseHeldEpisodes forgets held episodes.
func (db *DB) ReleaseHeldEpisodes(ctx context.Context, mappingID int64, episodeIDs []int) error {
	return db.eachHeld(ctx, episodeIDs, func(q *sqlcgen.Queries, id int) error {
		return q.DeleteHeldEpisode(ctx, sqlcgen.DeleteHeldEpisodeParams{MappingID: mappingID, SonarrEpisodeID: int64(id)})
	})
}

func (db *DB) eachHeld(ctx context.Context, episodeIDs []int, fn func(*sqlcgen.Queries, int) error) error {
	if len(episodeIDs) == 0 {
		return nil
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	qtx := db.q.WithTx(tx)
	for _, id := range episodeIDs {
		if err := fn(qtx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
