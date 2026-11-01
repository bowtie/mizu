package database

import (
	"context"
	"strings"
	"time"

	"mizu/internal/database/sqlcgen"
	"mizu/internal/models"

	_ "modernc.org/sqlite"

	"github.com/rs/zerolog/log"
)

func (db *DB) CreateSyncLog(ctx context.Context, triggerType string) (int64, error) {
	row, err := db.q.CreateSyncLog(ctx, triggerType)
	if err != nil {
		return 0, err
	}
	// Retention: bound the activity feed by dropping logs past the keep
	// window. Done here so every log-producing path (sync, resync, AFL
	// cache refresh) is covered automatically. Best-effort — a failed
	// prune must not fail the log creation.
	if n, perr := db.q.PruneSyncLogs(ctx, SyncLogRetentionCount); perr != nil {
		log.Warn().Err(perr).Msg("failed to prune old sync logs")
	} else if n > 0 {
		log.Debug().Int64("pruned", n).Msg("pruned old sync logs")
	}
	return row.ID, nil
}

func (db *DB) CompleteSyncLog(ctx context.Context, id int64, status string, showsProcessed, episodesUpdated int, errors string) error {
	if id == 0 {
		return nil
	}
	now := time.Now()
	return db.q.CompleteSyncLog(ctx, sqlcgen.CompleteSyncLogParams{
		CompletedAt:     &now,
		Status:          status,
		ShowsProcessed:  int64(showsProcessed),
		EpisodesUpdated: int64(episodesUpdated),
		Errors:          errors,
		ID:              id,
	})
}

// SyncLogRetentionCount is how many recent sync logs to keep. Older
// rows are pruned automatically as new logs are created.
const SyncLogRetentionCount = 200

// SweepStaleRunningSyncLogs marks logs still in 'running' state as
// failed. Called at startup: any row left running by a previous
// process's shutdown can never complete, and a stuck 'running' row also
// breaks the dashboard's next-sync estimate.
func (db *DB) SweepStaleRunningSyncLogs(ctx context.Context) (int64, error) {
	return db.q.FailStaleRunningSyncLogs(ctx)
}

func (db *DB) ReplaceSyncLogItems(ctx context.Context, logID int64, items []models.SyncLogItem) error {
	if logID == 0 {
		return nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := db.q.WithTx(tx)

	if err := qtx.DeleteSyncLogItemsByLog(ctx, logID); err != nil {
		return err
	}

	for _, item := range items {
		if err := qtx.InsertSyncLogItem(ctx, sqlcgen.InsertSyncLogItemParams{
			SyncLogID:       logID,
			MappingID:       item.MappingID,
			SonarrTitle:     item.SonarrTitle,
			Status:          item.Status,
			EpisodesChanged: int64(item.EpisodesChanged),
			Summary:         item.Summary,
			Error:           item.Error,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) GetLatestSyncLog(ctx context.Context) (models.SyncLog, error) {
	row, err := db.q.GetLatestSyncLog(ctx)
	if err != nil {
		return models.SyncLog{}, err
	}
	return sqlcSyncLogToModel(row), nil
}

func (db *DB) GetSyncLogsPage(ctx context.Context, limit, offset int) (models.SyncLogPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := db.q.GetSyncLogsPage(ctx, sqlcgen.GetSyncLogsPageParams{
		Limit:  int64(limit + 1),
		Offset: int64(offset),
	})
	if err != nil {
		return models.SyncLogPage{}, err
	}

	logs := make([]models.SyncLog, 0, len(rows))
	for _, r := range rows {
		logs = append(logs, sqlcSyncLogToModel(r))
	}

	page := models.SyncLogPage{}
	if len(logs) > limit {
		page.HasMore = true
		logs = logs[:limit]
	}
	page.Items = logs
	page.NextOffset = offset + len(logs)
	if len(logs) == 0 {
		return page, nil
	}

	logIDs := make([]int64, len(page.Items))
	for i := range page.Items {
		logIDs[i] = page.Items[i].ID
	}
	itemsByLog, err := db.getSyncLogItemsBatch(ctx, logIDs)
	if err != nil {
		return models.SyncLogPage{}, err
	}
	for i := range page.Items {
		page.Items[i].Items = itemsByLog[page.Items[i].ID]
	}

	return page, nil
}

func (db *DB) GetSyncLogItems(ctx context.Context, logIDs []int64) (map[int64][]models.SyncLogItem, error) {
	return db.getSyncLogItemsBatch(ctx, logIDs)
}

// getSyncLogItemsBatch fetches sync log items for multiple log IDs in a single
// query, avoiding N+1 patterns.
func (db *DB) getSyncLogItemsBatch(ctx context.Context, logIDs []int64) (map[int64][]models.SyncLogItem, error) {
	itemsByLog := make(map[int64][]models.SyncLogItem, len(logIDs))
	if len(logIDs) == 0 {
		return itemsByLog, nil
	}

	placeholders := make([]string, len(logIDs))
	args := make([]any, len(logIDs))
	for i, id := range logIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "SELECT id, sync_log_id, mapping_id, sonarr_title, status, episodes_changed, summary, error " +
		"FROM sync_log_items WHERE sync_log_id IN (" + strings.Join(placeholders, ",") + ") " +
		"ORDER BY sonarr_title COLLATE NOCASE ASC"

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.SyncLogItem
		var syncLogID int64
		var episodesChanged int64
		if err := rows.Scan(
			&item.ID,
			&syncLogID,
			&item.MappingID,
			&item.SonarrTitle,
			&item.Status,
			&episodesChanged,
			&item.Summary,
			&item.Error,
		); err != nil {
			return nil, err
		}
		item.EpisodesChanged = int(episodesChanged)
		itemsByLog[syncLogID] = append(itemsByLog[syncLogID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return itemsByLog, nil
}

// ── Settings ──
