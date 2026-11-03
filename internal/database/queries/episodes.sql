-- name: GetEpisodesByMapping :many
SELECT
    episode_num, title, type,
    sonarr_episode_id, season_number, sonarr_episode_number,
    absolute_episode_number, monitored, air_date, not_on_afl
FROM episode_info
WHERE mapping_id = ?
ORDER BY episode_num ASC;

-- name: GetEpisodeVersion :one
SELECT
    COUNT(*) AS count,
    COALESCE(SUM(monitored), 0) AS monitored_sum,
    COALESCE(MAX(last_scraped), '') AS max_scraped
FROM episode_info
WHERE mapping_id = ?;

-- name: DeleteEpisodesByMapping :exec
DELETE FROM episode_info WHERE mapping_id = ?;

-- name: InsertEpisode :exec
INSERT INTO episode_info (
    mapping_id, episode_num, title, type, last_scraped,
    sonarr_episode_id, season_number, sonarr_episode_number,
    absolute_episode_number, monitored, air_date, not_on_afl
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateEpisodeMonitor :exec
UPDATE episode_info SET monitored = ? WHERE mapping_id = ? AND sonarr_episode_id = ?;

-- name: GetFillerStats :one
SELECT
    COALESCE(SUM(CASE WHEN type = 'canon' THEN 1 ELSE 0 END), 0) AS canon,
    COALESCE(SUM(CASE WHEN type = 'mixed_canon' THEN 1 ELSE 0 END), 0) AS mixed,
    COALESCE(SUM(CASE WHEN type = 'anime_canon' THEN 1 ELSE 0 END), 0) AS anime_canon,
    COALESCE(SUM(CASE WHEN type = 'filler' THEN 1 ELSE 0 END), 0) AS filler,
    COUNT(*) AS total
FROM episode_info;
