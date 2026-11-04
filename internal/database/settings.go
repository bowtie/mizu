package database

import (
	"context"
	"database/sql"
	"errors"

	"mizu/internal/database/sqlcgen"

	_ "modernc.org/sqlite"
)

func (db *DB) GetSetting(ctx context.Context, key string) (string, error) {
	row, err := db.q.GetSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return row.Value, err
}

// GetAllSettings returns all stored settings in a single query.
func (db *DB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.q.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	settings := make(map[string]string, len(rows))
	for _, r := range rows {
		settings[r.Key] = r.Value
	}
	return settings, nil
}

// SetSettings upserts all provided settings in a single transaction so a
// partial failure can't leave the DB with a mix of old and new values.
func (db *DB) SetSettings(ctx context.Context, values map[string]string) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := db.q.WithTx(tx)
	for k, v := range values {
		if err := qtx.SetSetting(ctx, sqlcgen.SetSettingParams{Key: k, Value: v}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetSettingIfEmpty writes value only when the key is missing or holds
// an empty string, reporting whether it did. The check and the write are
// one statement, so two concurrent callers can't both succeed.
func (db *DB) SetSettingIfEmpty(ctx context.Context, key, value string) (bool, error) {
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO app_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value WHERE app_settings.value = ''`,
		key, value)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	return db.q.SetSetting(ctx, sqlcgen.SetSettingParams{
		Key:   key,
		Value: value,
	})
}

// ── AFL cache ──
