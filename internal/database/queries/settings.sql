-- name: GetSetting :one
SELECT key, value FROM app_settings WHERE key = ?;

-- name: SetSetting :exec
INSERT INTO app_settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: GetAllSettings :many
SELECT key, value FROM app_settings;
