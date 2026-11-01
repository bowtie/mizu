-- name: CreateSyncLog :one
INSERT INTO sync_logs (trigger_type) VALUES (?)
RETURNING id, started_at, completed_at, status, shows_processed, episodes_updated, errors, trigger_type;

-- name: CompleteSyncLog :exec
UPDATE sync_logs
SET completed_at = ?, status = ?, shows_processed = ?, episodes_updated = ?, errors = ?
WHERE id = ?;

-- name: GetLatestSyncLog :one
SELECT id, started_at, completed_at, status, shows_processed,
       episodes_updated, errors, trigger_type
FROM sync_logs
WHERE completed_at IS NOT NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: GetSyncLogsPage :many
SELECT id, started_at, completed_at, status, shows_processed,
       episodes_updated, errors, trigger_type
FROM sync_logs
ORDER BY started_at DESC
LIMIT ? OFFSET ?;

-- name: DeleteSyncLogItemsByLog :exec
DELETE FROM sync_log_items WHERE sync_log_id = ?;

-- name: InsertSyncLogItem :exec
INSERT INTO sync_log_items (sync_log_id, mapping_id, sonarr_title, status, episodes_changed, summary, error)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetSyncLogItemsByLogID :many
SELECT id, sync_log_id, mapping_id, sonarr_title, status, episodes_changed, summary, error
FROM sync_log_items
WHERE sync_log_id = ?
ORDER BY sonarr_title COLLATE NOCASE ASC;

-- name: PruneSyncLogs :execrows
-- Keeps the newest `?` logs (by start time); sync_log_items rows cascade.
DELETE FROM sync_logs WHERE id NOT IN (
    SELECT id FROM sync_logs ORDER BY started_at DESC, id DESC LIMIT ?
);

-- name: FailStaleRunningSyncLogs :execrows
-- Startup sweep: a shutdown mid-sync cancels the context before
-- CompleteSyncLog runs, leaving rows stuck in 'running' forever.
UPDATE sync_logs
SET status = 'failed',
    completed_at = datetime('now'),
    errors = CASE WHEN errors = '' THEN 'interrupted by restart'
                  ELSE errors || '; interrupted by restart' END
WHERE status = 'running';
