package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"mizu/internal/database/sqlcgen"
	"mizu/internal/models"

	_ "modernc.org/sqlite"

	"github.com/rs/zerolog/log"
)

func (db *DB) GetCachedAFLShows(ctx context.Context) ([]models.AFLShow, time.Time, error) {
	rows, err := db.q.GetAllCachedAFLShows(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}

	shows := make([]models.AFLShow, 0, len(rows))
	var refreshedAt time.Time
	for _, r := range rows {
		shows = append(shows, models.AFLShow{
			Title: r.Title,
			Slug:  r.Slug,
			URL:   r.Url,
		})
		if refreshedAt.IsZero() || r.RefreshedAt.After(refreshedAt) {
			refreshedAt = r.RefreshedAt
		}
	}
	return shows, refreshedAt, nil
}

func (db *DB) ReplaceCachedAFLShows(ctx context.Context, shows []models.AFLShow, refreshedAt time.Time) ([]string, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := db.q.WithTx(tx)

	// Collect existing slugs before deleting so we can detect new ones.
	existingSlugs, err := qtx.GetAllCachedAFLSlugs(ctx)
	if err != nil {
		return nil, err
	}
	existingSet := make(map[string]bool, len(existingSlugs))
	for _, s := range existingSlugs {
		existingSet[s] = true
	}

	if err := qtx.DeleteAllCachedAFLShows(ctx); err != nil {
		return nil, err
	}

	var newSlugs []string
	for _, show := range shows {
		if err := qtx.InsertCachedAFLShow(ctx, sqlcgen.InsertCachedAFLShowParams{
			Slug:        show.Slug,
			Title:       show.Title,
			Url:         show.URL,
			RefreshedAt: refreshedAt,
		}); err != nil {
			return nil, err
		}
		if !existingSet[show.Slug] {
			newSlugs = append(newSlugs, show.Slug)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if err := db.SetSetting(ctx, "afl_show_cache_refreshed_at", refreshedAt.UTC().Format(time.RFC3339)); err != nil {
		log.Warn().Err(err).Msg("failed to persist afl_show_cache_refreshed_at")
	}

	return newSlugs, nil
}

func (db *DB) GetAFLShowCacheStatus(ctx context.Context) (int, time.Time, error) {
	count, err := db.q.CountCachedAFLShows(ctx)
	if err != nil {
		return 0, time.Time{}, err
	}

	value, err := db.GetSetting(ctx, "afl_show_cache_refreshed_at")
	if err != nil || value == "" {
		return int(count), time.Time{}, err
	}

	refreshedAt, parseErr := time.Parse(time.RFC3339, value)
	if parseErr != nil {
		return int(count), time.Time{}, nil
	}
	return int(count), refreshedAt, nil
}

// ── Filler stats ──

func (db *DB) GetFillerStats(ctx context.Context) (canon, mixed, animeCanon, filler, total int, err error) {
	row, err := db.q.GetFillerStats(ctx)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	canon = toInt(row.Canon)
	mixed = toInt(row.Mixed)
	animeCanon = toInt(row.AnimeCanon)
	filler = toInt(row.Filler)
	total = int(row.Total)
	return
}

// ── Sonarr cache ──

func (db *DB) GetCachedSonarrSeries(ctx context.Context, seriesID int, maxAge time.Duration) (*models.SonarrSeries, bool) {
	row, err := db.q.GetCachedSonarrSeries(ctx, int64(seriesID))
	if err != nil {
		return nil, false
	}
	if time.Since(row.CachedAt) > maxAge {
		return nil, false
	}
	var series models.SonarrSeries
	if err := json.Unmarshal([]byte(row.DataJson), &series); err != nil {
		return nil, false
	}
	return &series, true
}

func (db *DB) SetCachedSonarrSeries(ctx context.Context, seriesID int, series *models.SonarrSeries) error {
	dataJSON, err := json.Marshal(series)
	if err != nil {
		return err
	}
	return db.q.SetCachedSonarrSeries(ctx, sqlcgen.SetCachedSonarrSeriesParams{
		SeriesID: int64(seriesID),
		DataJson: string(dataJSON),
		CachedAt: time.Now(),
	})
}

// ── Helpers ──
