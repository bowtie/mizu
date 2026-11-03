-- name: GetHeldEpisodes :many
SELECT sonarr_episode_id, held_at FROM held_episodes WHERE mapping_id = ?;

-- name: InsertHeldEpisode :exec
INSERT OR IGNORE INTO held_episodes (mapping_id, sonarr_episode_id, held_at) VALUES (?, ?, ?);

-- name: DeleteHeldEpisode :exec
DELETE FROM held_episodes WHERE mapping_id = ? AND sonarr_episode_id = ?;
