-- name: GetAllCachedAFLShows :many
SELECT title, slug, url, refreshed_at
FROM afl_show_cache
ORDER BY title COLLATE NOCASE ASC;

-- name: GetAllCachedAFLSlugs :many
SELECT slug FROM afl_show_cache;

-- name: DeleteAllCachedAFLShows :exec
DELETE FROM afl_show_cache;

-- name: InsertCachedAFLShow :exec
INSERT INTO afl_show_cache (slug, title, url, refreshed_at)
VALUES (?, ?, ?, ?);

-- name: CountCachedAFLShows :one
SELECT COUNT(*) FROM afl_show_cache;

-- name: GetCachedSonarrSeries :one
SELECT data_json, cached_at
FROM sonarr_series_cache
WHERE series_id = ?;

-- name: SetCachedSonarrSeries :exec
INSERT INTO sonarr_series_cache (series_id, data_json, cached_at) VALUES (?, ?, ?)
ON CONFLICT(series_id) DO UPDATE SET data_json = excluded.data_json, cached_at = excluded.cached_at;
