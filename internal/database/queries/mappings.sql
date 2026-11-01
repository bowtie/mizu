-- name: GetAllMappings :many
SELECT
    id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
    auto_matched, match_confidence, filler_action, mixed_action,
    anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
    artwork_source, artwork_poster_path, artwork_banner_path, artwork_logo_path,
    overview, created_at, updated_at
FROM show_mappings
ORDER BY sonarr_title ASC;

-- name: GetMappingByID :one
SELECT
    id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
    auto_matched, match_confidence, filler_action, mixed_action,
    anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
    artwork_source, artwork_poster_path, artwork_banner_path, artwork_logo_path,
    overview, created_at, updated_at
FROM show_mappings
WHERE id = ?;

-- name: GetMappingBySonarrID :one
SELECT
    id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
    auto_matched, match_confidence, filler_action, mixed_action,
    anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
    artwork_source, artwork_poster_path, artwork_banner_path, artwork_logo_path,
    overview, created_at, updated_at
FROM show_mappings
WHERE sonarr_series_id = ?;

-- name: DeleteMapping :exec
DELETE FROM show_mappings WHERE id = ?;

-- name: UpsertMapping :exec
INSERT INTO show_mappings (
    sonarr_series_id, sonarr_title, afl_slug, afl_title,
    auto_matched, match_confidence, filler_action, mixed_action,
    anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
    overview, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(sonarr_series_id) DO UPDATE SET
    sonarr_title = excluded.sonarr_title,
    afl_slug = excluded.afl_slug,
    afl_title = excluded.afl_title,
    auto_matched = excluded.auto_matched,
    match_confidence = excluded.match_confidence,
    filler_action = excluded.filler_action,
    mixed_action = excluded.mixed_action,
    anime_canon_action = excluded.anime_canon_action,
    enabled = excluded.enabled,
    start_mode = excluded.start_mode,
    watermark_mode = excluded.watermark_mode,
    sync_watermark = excluded.sync_watermark,
    overview = excluded.overview,
    updated_at = excluded.updated_at;

-- name: GetMappingIDBySonarrID :one
SELECT id FROM show_mappings WHERE sonarr_series_id = ?;

-- name: UpdateArtworkFiles :exec
UPDATE show_mappings
SET artwork_poster_path = ?, artwork_banner_path = ?, artwork_logo_path = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateArtworkSource :exec
UPDATE show_mappings
SET artwork_source = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateOverview :exec
UPDATE show_mappings
SET overview = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateWatermarkState :exec
UPDATE show_mappings
SET sync_watermark = ?, watermark_mode = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
