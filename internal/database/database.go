package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mizu/internal/models"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// New opens or creates a SQLite database at the given path.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS show_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sonarr_series_id INTEGER NOT NULL UNIQUE,
			sonarr_title TEXT NOT NULL,
			afl_slug TEXT NOT NULL,
			afl_title TEXT NOT NULL,
			auto_matched INTEGER NOT NULL DEFAULT 1,
			match_confidence REAL NOT NULL DEFAULT 0.0,
			filler_action TEXT NOT NULL DEFAULT 'unmonitor',
			mixed_action TEXT NOT NULL DEFAULT 'monitor',
			anime_canon_action TEXT NOT NULL DEFAULT 'monitor',
			enabled INTEGER NOT NULL DEFAULT 1,
			start_mode TEXT NOT NULL DEFAULT 'from_current_progress',
			watermark_mode TEXT NOT NULL DEFAULT 'auto',
			sync_watermark INTEGER NOT NULL DEFAULT 0,
			anilist_poster_url TEXT NOT NULL DEFAULT '',
			anilist_banner_url TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS episode_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mapping_id INTEGER NOT NULL,
			episode_num INTEGER NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'unknown',
			last_scraped DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			sonarr_episode_id INTEGER NOT NULL DEFAULT 0,
			season_number INTEGER NOT NULL DEFAULT 0,
			sonarr_episode_number INTEGER NOT NULL DEFAULT 0,
			absolute_episode_number INTEGER,
			monitored INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (mapping_id) REFERENCES show_mappings(id) ON DELETE CASCADE,
			UNIQUE(mapping_id, episode_num)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			status TEXT NOT NULL DEFAULT 'running',
			shows_processed INTEGER NOT NULL DEFAULT 0,
			episodes_updated INTEGER NOT NULL DEFAULT 0,
			errors TEXT NOT NULL DEFAULT '',
			trigger_type TEXT NOT NULL DEFAULT 'manual'
		)`,
		`CREATE TABLE IF NOT EXISTS sync_log_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sync_log_id INTEGER NOT NULL,
			mapping_id INTEGER NOT NULL DEFAULT 0,
			sonarr_title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'checked',
			episodes_changed INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (sync_log_id) REFERENCES sync_logs(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS afl_show_cache (
			slug TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			refreshed_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sonarr_series_cache (
			series_id INTEGER PRIMARY KEY,
			data_json TEXT NOT NULL,
			cached_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_episode_info_mapping ON episode_info(mapping_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_logs_started ON sync_logs(started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_afl_show_cache_title ON afl_show_cache(title COLLATE NOCASE)`,
	}

	for _, alterSQL := range []string{
		"ALTER TABLE show_mappings ADD COLUMN anime_canon_action TEXT NOT NULL DEFAULT 'monitor'",
		"ALTER TABLE show_mappings ADD COLUMN sync_watermark INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE show_mappings ADD COLUMN start_mode TEXT NOT NULL DEFAULT 'from_current_progress'",
		"ALTER TABLE show_mappings ADD COLUMN watermark_mode TEXT NOT NULL DEFAULT 'auto'",
		"ALTER TABLE episode_info ADD COLUMN sonarr_episode_id INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE episode_info ADD COLUMN season_number INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE episode_info ADD COLUMN sonarr_episode_number INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE episode_info ADD COLUMN absolute_episode_number INTEGER",
		"ALTER TABLE episode_info ADD COLUMN monitored INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE show_mappings ADD COLUMN anilist_poster_url TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE show_mappings ADD COLUMN anilist_banner_url TEXT NOT NULL DEFAULT ''",
	} {
		db.conn.Exec(alterSQL)
	}

	for _, dropSQL := range []string{
		"ALTER TABLE show_mappings DROP COLUMN IF EXISTS episode_cutoff",
		"ALTER TABLE show_mappings DROP COLUMN IF EXISTS season_cutoff",
		"ALTER TABLE show_mappings DROP COLUMN IF EXISTS episode_offset",
		"ALTER TABLE episode_info DROP COLUMN IF EXISTS arc_name",
	} {
		db.conn.Exec(dropSQL)
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return nil
}

func (db *DB) GetAllMappings(ctx context.Context) ([]models.ShowMapping, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
		       auto_matched, match_confidence, filler_action, mixed_action,
		       anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
		       anilist_poster_url, anilist_banner_url, created_at, updated_at
		FROM show_mappings ORDER BY sonarr_title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []models.ShowMapping
	for rows.Next() {
		var m models.ShowMapping
		err := rows.Scan(
			&m.ID, &m.SonarrSeriesID, &m.SonarrTitle, &m.AFLSlug, &m.AFLTitle,
			&m.AutoMatched, &m.MatchConfidence, &m.FillerAction, &m.MixedAction,
			&m.AnimeCanonAction, &m.Enabled, &m.StartMode, &m.WatermarkMode, &m.SyncWatermark,
			&m.AnilistPosterURL, &m.AnilistBannerURL, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, normalizeMapping(m))
	}
	return mappings, rows.Err()
}

func (db *DB) GetMappingByID(ctx context.Context, id int64) (*models.ShowMapping, error) {
	var m models.ShowMapping
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
		       auto_matched, match_confidence, filler_action, mixed_action,
		       anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
		       anilist_poster_url, anilist_banner_url, created_at, updated_at
		FROM show_mappings WHERE id = ?`, id).Scan(
		&m.ID, &m.SonarrSeriesID, &m.SonarrTitle, &m.AFLSlug, &m.AFLTitle,
		&m.AutoMatched, &m.MatchConfidence, &m.FillerAction, &m.MixedAction,
		&m.AnimeCanonAction, &m.Enabled, &m.StartMode, &m.WatermarkMode, &m.SyncWatermark,
		&m.AnilistPosterURL, &m.AnilistBannerURL, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m = normalizeMapping(m)
	return &m, nil
}

func (db *DB) GetMappingBySonarrID(ctx context.Context, seriesID int) (*models.ShowMapping, error) {
	var m models.ShowMapping
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
		       auto_matched, match_confidence, filler_action, mixed_action,
		       anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark,
		       anilist_poster_url, anilist_banner_url, created_at, updated_at
		FROM show_mappings WHERE sonarr_series_id = ?`, seriesID).Scan(
		&m.ID, &m.SonarrSeriesID, &m.SonarrTitle, &m.AFLSlug, &m.AFLTitle,
		&m.AutoMatched, &m.MatchConfidence, &m.FillerAction, &m.MixedAction,
		&m.AnimeCanonAction, &m.Enabled, &m.StartMode, &m.WatermarkMode, &m.SyncWatermark,
		&m.AnilistPosterURL, &m.AnilistBannerURL, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m = normalizeMapping(m)
	return &m, nil
}

func (db *DB) UpsertMapping(ctx context.Context, m *models.ShowMapping) error {
	if m.StartMode == "" {
		m.StartMode = models.StartModeCurrentProgress
	}
	if m.WatermarkMode == "" {
		if m.StartMode == models.StartModeFromBeginning {
			m.WatermarkMode = models.WatermarkModeManual
		} else {
			m.WatermarkMode = models.WatermarkModeAuto
		}
	}
	if m.SyncWatermark < 0 {
		m.SyncWatermark = 0
	}
	*m = normalizeMapping(*m)
	now := time.Now()
	m.UpdatedAt = now

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO show_mappings (sonarr_series_id, sonarr_title, afl_slug, afl_title,
		    auto_matched, match_confidence, filler_action, mixed_action,
		    anime_canon_action, enabled, start_mode, watermark_mode, sync_watermark, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		    updated_at = excluded.updated_at`,
		m.SonarrSeriesID, m.SonarrTitle, m.AFLSlug, m.AFLTitle,
		m.AutoMatched, m.MatchConfidence, m.FillerAction, m.MixedAction,
		m.AnimeCanonAction, m.Enabled, m.StartMode, m.WatermarkMode, m.SyncWatermark, now, now,
	)
	return err
}

func (db *DB) UpdateAnilistArtwork(ctx context.Context, mappingID int64, posterURL, bannerURL string) error {
	_, err := db.conn.ExecContext(ctx, `UPDATE show_mappings SET anilist_poster_url = ?, anilist_banner_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, posterURL, bannerURL, mappingID)
	return err
}

func (db *DB) UpdateWatermarkState(ctx context.Context, mappingID int64, watermark int, mode models.WatermarkMode) error {
	if watermark < 0 {
		watermark = 0
	}
	if mode == "" {
		mode = models.WatermarkModeAuto
	}
	_, err := db.conn.ExecContext(ctx, `UPDATE show_mappings SET sync_watermark = ?, watermark_mode = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, watermark, mode, mappingID)
	return err
}

func (db *DB) DeleteMapping(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM show_mappings WHERE id = ?`, id)
	return err
}

func (db *DB) GetEpisodes(ctx context.Context, mappingID int64) ([]models.EpisodeInfo, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, mapping_id, episode_num, title, type, last_scraped,
		       sonarr_episode_id, season_number, sonarr_episode_number,
		       absolute_episode_number, monitored
		FROM episode_info WHERE mapping_id = ? ORDER BY episode_num ASC`, mappingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []models.EpisodeInfo
	for rows.Next() {
		var e models.EpisodeInfo
		var absNum sql.NullInt64
		err := rows.Scan(&e.ID, &e.MappingID, &e.EpisodeNum, &e.Title, &e.Type, &e.LastScraped,
			&e.SonarrEpisodeID, &e.SeasonNumber, &e.SonarrEpisodeNumber,
			&absNum, &e.Monitored)
		if err != nil {
			return nil, err
		}
		if absNum.Valid {
			val := int(absNum.Int64)
			e.AbsoluteEpisodeNumber = &val
		}
		episodes = append(episodes, e)
	}
	return episodes, rows.Err()
}

func (db *DB) BulkUpsertEpisodes(ctx context.Context, episodes []models.EpisodeInfo) error {
	if len(episodes) == 0 {
		return nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO episode_info (mapping_id, episode_num, title, type, last_scraped,
			sonarr_episode_id, season_number, sonarr_episode_number, absolute_episode_number, monitored)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(mapping_id, episode_num) DO UPDATE SET
		    title = excluded.title,
		    type = excluded.type,
		    last_scraped = excluded.last_scraped,
		    sonarr_episode_id = excluded.sonarr_episode_id,
		    season_number = excluded.season_number,
		    sonarr_episode_number = excluded.sonarr_episode_number,
		    absolute_episode_number = excluded.absolute_episode_number,
		    monitored = excluded.monitored`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, e := range episodes {
		var absNum any
		if e.AbsoluteEpisodeNumber != nil {
			absNum = *e.AbsoluteEpisodeNumber
		}
		_, err := stmt.ExecContext(ctx, e.MappingID, e.EpisodeNum, e.Title, e.Type, now,
			e.SonarrEpisodeID, e.SeasonNumber, e.SonarrEpisodeNumber, absNum, e.Monitored)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) ReplaceEpisodesForMapping(ctx context.Context, mappingID int64, episodes []models.EpisodeInfo) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM episode_info WHERE mapping_id = ?`, mappingID); err != nil {
		return err
	}

	if len(episodes) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO episode_info (mapping_id, episode_num, title, type, last_scraped,
			sonarr_episode_id, season_number, sonarr_episode_number, absolute_episode_number, monitored)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, e := range episodes {
		var absNum any
		if e.AbsoluteEpisodeNumber != nil {
			absNum = *e.AbsoluteEpisodeNumber
		}
		_, err := stmt.ExecContext(ctx, mappingID, e.EpisodeNum, e.Title, e.Type, now,
			e.SonarrEpisodeID, e.SeasonNumber, e.SonarrEpisodeNumber, absNum, e.Monitored)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) CreateSyncLog(ctx context.Context, triggerType string) (int64, error) {
	result, err := db.conn.ExecContext(ctx, `
		INSERT INTO sync_logs (trigger_type) VALUES (?)`, triggerType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) CompleteSyncLog(ctx context.Context, id int64, status string, showsProcessed, episodesUpdated int, errors string) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE sync_logs SET completed_at = ?, status = ?, shows_processed = ?,
		    episodes_updated = ?, errors = ?
		WHERE id = ?`, time.Now(), status, showsProcessed, episodesUpdated, errors, id)
	return err
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

	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_log_items WHERE sync_log_id = ?`, logID); err != nil {
		return err
	}
	if len(items) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO sync_log_items (sync_log_id, mapping_id, sonarr_title, status, episodes_changed, summary, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, logID, item.MappingID, item.SonarrTitle, item.Status, item.EpisodesChanged, item.Summary, item.Error); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetLatestSyncLog returns the most recent completed sync log, or sql.ErrNoRows if none.
func (db *DB) GetLatestSyncLog(ctx context.Context) (models.SyncLog, error) {
	var log models.SyncLog
	row := db.conn.QueryRowContext(ctx, `
		SELECT id, started_at, completed_at, status, shows_processed,
		       episodes_updated, errors, trigger_type
		FROM sync_logs WHERE completed_at IS NOT NULL
		ORDER BY started_at DESC LIMIT 1`)
	err := row.Scan(
		&log.ID, &log.StartedAt, &log.CompletedAt, &log.Status,
		&log.ShowsProcessed, &log.EpisodesUpdated, &log.Errors, &log.Trigger,
	)
	return log, err
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

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, started_at, completed_at, status, shows_processed,
		       episodes_updated, errors, trigger_type
		FROM sync_logs ORDER BY started_at DESC LIMIT ? OFFSET ?`, limit+1, offset)
	if err != nil {
		return models.SyncLogPage{}, err
	}
	defer rows.Close()

	logs := make([]models.SyncLog, 0, limit+1)
	for rows.Next() {
		var l models.SyncLog
		var completedAt sql.NullTime
		if err := rows.Scan(&l.ID, &l.StartedAt, &completedAt, &l.Status, &l.ShowsProcessed, &l.EpisodesUpdated, &l.Errors, &l.Trigger); err != nil {
			return models.SyncLogPage{}, err
		}
		if completedAt.Valid {
			l.CompletedAt = completedAt.Time
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return models.SyncLogPage{}, err
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

	logIDs := make([]int64, 0, len(logs))
	for _, log := range logs {
		logIDs = append(logIDs, log.ID)
	}
	itemsByLog, err := db.GetSyncLogItems(ctx, logIDs)
	if err != nil {
		return models.SyncLogPage{}, err
	}
	for i := range page.Items {
		page.Items[i].Items = itemsByLog[page.Items[i].ID]
	}

	return page, nil
}

func (db *DB) GetSyncLogItems(ctx context.Context, logIDs []int64) (map[int64][]models.SyncLogItem, error) {
	itemsByLog := make(map[int64][]models.SyncLogItem, len(logIDs))
	if len(logIDs) == 0 {
		return itemsByLog, nil
	}

	placeholders := make([]string, len(logIDs))
	args := make([]interface{}, len(logIDs))
	for i, id := range logIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT id, sync_log_id, mapping_id, sonarr_title, status, episodes_changed, summary, error
		FROM sync_log_items
		WHERE sync_log_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY sync_log_id DESC, sonarr_title COLLATE NOCASE ASC`
	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.SyncLogItem
		if err := rows.Scan(&item.ID, &item.SyncLogID, &item.MappingID, &item.SonarrTitle, &item.Status, &item.EpisodesChanged, &item.Summary, &item.Error); err != nil {
			return nil, err
		}
		itemsByLog[item.SyncLogID] = append(itemsByLog[item.SyncLogID], item)
	}
	return itemsByLog, rows.Err()
}

func (db *DB) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := db.conn.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (db *DB) GetCachedAFLShows(ctx context.Context) ([]models.AFLShow, time.Time, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT title, slug, url, refreshed_at
		FROM afl_show_cache
		ORDER BY title COLLATE NOCASE ASC`)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var shows []models.AFLShow
	var refreshedAt time.Time
	for rows.Next() {
		var show models.AFLShow
		var rowRefreshedAt time.Time
		if err := rows.Scan(&show.Title, &show.Slug, &show.URL, &rowRefreshedAt); err != nil {
			return nil, time.Time{}, err
		}
		if refreshedAt.IsZero() || rowRefreshedAt.After(refreshedAt) {
			refreshedAt = rowRefreshedAt
		}
		shows = append(shows, show)
	}
	return shows, refreshedAt, rows.Err()
}

func (db *DB) ReplaceCachedAFLShows(ctx context.Context, shows []models.AFLShow, refreshedAt time.Time) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM afl_show_cache`); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO afl_show_cache (slug, title, url, refreshed_at)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, show := range shows {
		if _, err := stmt.ExecContext(ctx, show.Slug, show.Title, show.URL, refreshedAt); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return db.SetSetting(ctx, "afl_show_cache_refreshed_at", refreshedAt.UTC().Format(time.RFC3339))
}

func (db *DB) GetAFLShowCacheStatus(ctx context.Context) (int, time.Time, error) {
	var count int
	if err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM afl_show_cache`).Scan(&count); err != nil {
		return 0, time.Time{}, err
	}

	value, err := db.GetSetting(ctx, "afl_show_cache_refreshed_at")
	if err != nil || value == "" {
		return count, time.Time{}, err
	}

	refreshedAt, parseErr := time.Parse(time.RFC3339, value)
	if parseErr != nil {
		return count, time.Time{}, nil
	}
	return count, refreshedAt, nil
}

func (db *DB) GetFillerStats(ctx context.Context) (canon, mixed, animeCanon, filler, total int, err error) {
	err = db.conn.QueryRowContext(ctx, `
		SELECT
			SUM(CASE WHEN type = 'canon' THEN 1 ELSE 0 END),
			SUM(CASE WHEN type = 'mixed_canon' THEN 1 ELSE 0 END),
			SUM(CASE WHEN type = 'anime_canon' THEN 1 ELSE 0 END),
			SUM(CASE WHEN type = 'filler' THEN 1 ELSE 0 END),
			COUNT(*)
		FROM episode_info`).Scan(&canon, &mixed, &animeCanon, &filler, &total)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, 0, nil
	}
	return
}

func (db *DB) GetCachedSonarrSeries(ctx context.Context, seriesID int, maxAge time.Duration) (*models.SonarrSeries, bool) {
	var dataJSON string
	var cachedAt time.Time
	err := db.conn.QueryRowContext(ctx,
		`SELECT data_json, cached_at FROM sonarr_series_cache WHERE series_id = ?`, seriesID).
		Scan(&dataJSON, &cachedAt)
	if err != nil {
		return nil, false
	}
	if time.Since(cachedAt) > maxAge {
		return nil, false
	}
	var series models.SonarrSeries
	if err := json.Unmarshal([]byte(dataJSON), &series); err != nil {
		return nil, false
	}
	return &series, true
}

func (db *DB) SetCachedSonarrSeries(ctx context.Context, seriesID int, series *models.SonarrSeries) error {
	dataJSON, err := json.Marshal(series)
	if err != nil {
		return err
	}
	_, err = db.conn.ExecContext(ctx,
		`INSERT INTO sonarr_series_cache (series_id, data_json, cached_at) VALUES (?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET data_json = excluded.data_json, cached_at = excluded.cached_at`,
		seriesID, string(dataJSON), time.Now())
	return err
}

func normalizeMapping(m models.ShowMapping) models.ShowMapping {
	if m.StartMode == "" {
		m.StartMode = models.StartModeCurrentProgress
	}
	if m.WatermarkMode == "" {
		if m.StartMode == models.StartModeFromBeginning {
			m.WatermarkMode = models.WatermarkModeManual
		} else {
			m.WatermarkMode = models.WatermarkModeAuto
		}
	}
	if m.SyncWatermark < 0 {
		m.SyncWatermark = 0
	}
	return m
}
