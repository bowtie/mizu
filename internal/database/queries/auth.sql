-- name: CreateSession :exec
INSERT INTO auth_sessions (token_hash, expires_at) VALUES (?, ?);

-- name: GetSessionExpiry :one
-- Expiry filtering happens in Go so the comparison uses the same
-- timestamp encoding the driver writes, not SQLite's datetime() text.
SELECT expires_at FROM auth_sessions WHERE token_hash = ?;

-- name: DeleteSession :exec
DELETE FROM auth_sessions WHERE token_hash = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM auth_sessions WHERE expires_at <= ?;

-- name: DeleteAllSessions :exec
DELETE FROM auth_sessions;
