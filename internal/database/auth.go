package database

import (
	"context"
	"time"

	"mizu/internal/database/sqlcgen"
)

// ── Auth sessions ──
// Sessions are stored keyed by SHA-256(token) so a database leak cannot
// be replayed as a valid cookie. The password hash itself lives in
// app_settings under auth.SettingPasswordHash.

// CreateSession persists a session token hash with the given expiry.
func (db *DB) CreateSession(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	return db.q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
}

// GetSessionExpiry returns the stored expiry for a session token hash.
// Unknown tokens return an error (sql.ErrNoRows); callers must check
// expiry themselves.
func (db *DB) GetSessionExpiry(ctx context.Context, tokenHash string) (time.Time, error) {
	return db.q.GetSessionExpiry(ctx, tokenHash)
}

// DeleteSession removes a single session; deleting an unknown token is
// not an error.
func (db *DB) DeleteSession(ctx context.Context, tokenHash string) error {
	return db.q.DeleteSession(ctx, tokenHash)
}

// DeleteExpiredSessions removes sessions expired before now and returns
// the count. The cutoff is passed from Go so the comparison matches the
// driver's timestamp encoding exactly.
func (db *DB) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	return db.q.DeleteExpiredSessions(ctx, now)
}

// DeleteAllSessions revokes every session (used on password change).
func (db *DB) DeleteAllSessions(ctx context.Context) error {
	return db.q.DeleteAllSessions(ctx)
}
