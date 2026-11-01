package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"mizu/internal/database/sqlcgen"
	"mizu/internal/models"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
)

// DB wraps the SQLite database connection and sqlc-generated queries.
type DB struct {
	conn *sql.DB
	q    *sqlcgen.Queries
}

// New opens or creates a SQLite database at the given path.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable foreign key constraints so ON DELETE CASCADE actually fires.
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	// WAL mode supports concurrent readers. Allow a small pool so that
	// parallel requests (e.g. getShow + getEpisodes on the detail page)
	// don't serialize behind a single connection. The _busy_timeout=5000
	// in the DSN handles write contention gracefully.
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(4)
	conn.SetConnMaxLifetime(0)

	db := &DB{conn: conn, q: sqlcgen.New(conn)}
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
	goose.SetLogger(goose.NopLogger())

	// Check if this is a pre-existing database (tables already exist
	// but goose_db_version doesn't). If so, mark all current migrations
	// as applied so goose doesn't try to re-create tables.
	var tableCount int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'goose_db_version'`).Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("checking existing tables: %w", err)
	}

	if tableCount > 0 {
		var gooseTableExists int
		err := db.conn.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name = 'goose_db_version'`).Scan(&gooseTableExists)
		if err != nil {
			return fmt.Errorf("checking goose table: %w", err)
		}
		if gooseTableExists == 0 {
			if _, err := db.conn.Exec(`
				CREATE TABLE goose_db_version (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					version_id INTEGER NOT NULL,
					is_applied INTEGER NOT NULL,
					tstamp TIMESTAMP DEFAULT (datetime('now'))
				)`); err != nil {
				return fmt.Errorf("creating goose_db_version: %w", err)
			}
			// goose expects a version 0 row to exist before it checks
			// for applied migrations. Without it, goose tries to recreate
			// the table and crashes.
			if _, err := db.conn.Exec(`
				INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, 1)`); err != nil {
				return fmt.Errorf("inserting goose zero version: %w", err)
			}
			if _, err := db.conn.Exec(`
				INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, 1)`); err != nil {
				return fmt.Errorf("marking initial migration as applied: %w", err)
			}
		}
	}

	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating sub-filesystem: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db.conn, subFS)
	if err != nil {
		return fmt.Errorf("creating goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("running goose migrations: %w", err)
	}

	return nil
}

// ── Conversion helpers ──

func sqlcMappingToModel(s sqlcgen.ShowMapping) models.ShowMapping {
	return models.ShowMapping{
		ID:                s.ID,
		SonarrSeriesID:    int(s.SonarrSeriesID),
		SonarrTitle:       s.SonarrTitle,
		AFLSlug:           s.AflSlug,
		AFLTitle:          s.AflTitle,
		AutoMatched:       s.AutoMatched != 0,
		MatchConfidence:   s.MatchConfidence,
		FillerAction:      models.FillerAction(s.FillerAction),
		MixedAction:       models.FillerAction(s.MixedAction),
		AnimeCanonAction:  models.FillerAction(s.AnimeCanonAction),
		Enabled:           s.Enabled != 0,
		StartMode:         models.StartMode(s.StartMode),
		WatermarkMode:     models.WatermarkMode(s.WatermarkMode),
		SyncWatermark:     int(s.SyncWatermark),
		ArtworkSource:     s.ArtworkSource,
		ArtworkPosterPath: s.ArtworkPosterPath,
		ArtworkBannerPath: s.ArtworkBannerPath,
		ArtworkLogoPath:   s.ArtworkLogoPath,
		ArtworkPosterURL:  s.ArtworkPosterUrl,
		ArtworkBannerURL:  s.ArtworkBannerUrl,
		ArtworkLogoURL:    s.ArtworkLogoUrl,
		Overview:          s.Overview,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

func sqlcEpisodeToModel(s sqlcgen.GetEpisodesByMappingRow) models.EpisodeInfo {
	e := models.EpisodeInfo{
		EpisodeNum:          int(s.EpisodeNum),
		Title:               s.Title,
		Type:                models.EpisodeType(s.Type),
		SonarrEpisodeID:     int(s.SonarrEpisodeID),
		SeasonNumber:        int(s.SeasonNumber),
		SonarrEpisodeNumber: int(s.SonarrEpisodeNumber),
		Monitored:           s.Monitored != 0,
	}
	if s.AbsoluteEpisodeNumber != nil {
		val := int(*s.AbsoluteEpisodeNumber)
		e.AbsoluteEpisodeNumber = &val
	}
	if s.AirDate != nil {
		d := models.DateOnly(*s.AirDate)
		e.AirDate = &d
	}
	return e
}

func sqlcSyncLogToModel(s sqlcgen.SyncLog) models.SyncLog {
	l := models.SyncLog{
		ID:              s.ID,
		StartedAt:       s.StartedAt,
		Status:          s.Status,
		ShowsProcessed:  int(s.ShowsProcessed),
		EpisodesUpdated: int(s.EpisodesUpdated),
		Errors:          s.Errors,
		Trigger:         s.TriggerType,
	}
	if s.CompletedAt != nil {
		l.CompletedAt = s.CompletedAt
	}
	return l
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

// ── Mappings ──

func (db *DB) GetAllMappings(ctx context.Context) ([]models.ShowMapping, error) {
	rows, err := db.q.GetAllMappings(ctx)
	if err != nil {
		return nil, err
	}
	mappings := make([]models.ShowMapping, 0, len(rows))
	for _, r := range rows {
		mappings = append(mappings, normalizeMapping(sqlcMappingToModel(r)))
	}
	return mappings, nil
}

func (db *DB) GetMappingByID(ctx context.Context, id int64) (*models.ShowMapping, error) {
	row, err := db.q.GetMappingByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := normalizeMapping(sqlcMappingToModel(row))
	return &m, nil
}

func (db *DB) GetMappingBySonarrID(ctx context.Context, seriesID int) (*models.ShowMapping, error) {
	row, err := db.q.GetMappingBySonarrID(ctx, int64(seriesID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := normalizeMapping(sqlcMappingToModel(row))
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

	err := db.q.UpsertMapping(ctx, sqlcgen.UpsertMappingParams{
		SonarrSeriesID:   int64(m.SonarrSeriesID),
		SonarrTitle:      m.SonarrTitle,
		AflSlug:          m.AFLSlug,
		AflTitle:         m.AFLTitle,
		AutoMatched:      boolToInt(m.AutoMatched),
		MatchConfidence:  m.MatchConfidence,
		FillerAction:     string(m.FillerAction),
		MixedAction:      string(m.MixedAction),
		AnimeCanonAction: string(m.AnimeCanonAction),
		Enabled:          boolToInt(m.Enabled),
		StartMode:        string(m.StartMode),
		WatermarkMode:    string(m.WatermarkMode),
		SyncWatermark:    int64(m.SyncWatermark),
		Overview:         m.Overview,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		return err
	}

	// Populate the ID on insert (autoincrement). On conflict (update),
	// LastInsertId returns 0 in modernc.org/sqlite, so fetch it if needed.
	if m.ID == 0 {
		id, err := db.q.GetMappingIDBySonarrID(ctx, int64(m.SonarrSeriesID))
		if err == nil {
			m.ID = id
		}
	}
	return nil
}

func (db *DB) UpdateArtwork(ctx context.Context, mappingID int64, posterPath, bannerPath, logoPath, posterURL, bannerURL, logoURL string) error {
	return db.q.UpdateArtwork(ctx, sqlcgen.UpdateArtworkParams{
		ArtworkPosterPath: posterPath,
		ArtworkBannerPath: bannerPath,
		ArtworkLogoPath:   logoPath,
		ArtworkPosterUrl:  posterURL,
		ArtworkBannerUrl:  bannerURL,
		ArtworkLogoUrl:    logoURL,
		ID:                mappingID,
	})
}

func (db *DB) UpdateArtworkSource(ctx context.Context, mappingID int64, source string) error {
	return db.q.UpdateArtworkSource(ctx, sqlcgen.UpdateArtworkSourceParams{
		ArtworkSource: source,
		ID:            mappingID,
	})
}

func (db *DB) UpdateOverview(ctx context.Context, mappingID int64, overview string) error {
	return db.q.UpdateOverview(ctx, sqlcgen.UpdateOverviewParams{
		Overview: overview,
		ID:       mappingID,
	})
}

func (db *DB) UpdateWatermarkState(ctx context.Context, mappingID int64, watermark int, mode models.WatermarkMode) error {
	if watermark < 0 {
		watermark = 0
	}
	if mode == "" {
		mode = models.WatermarkModeAuto
	}
	return db.q.UpdateWatermarkState(ctx, sqlcgen.UpdateWatermarkStateParams{
		SyncWatermark: int64(watermark),
		WatermarkMode: string(mode),
		ID:            mappingID,
	})
}

func (db *DB) DeleteMapping(ctx context.Context, id int64) error {
	return db.q.DeleteMapping(ctx, id)
}

// ── Episodes ──

func (db *DB) GetEpisodes(ctx context.Context, mappingID int64) ([]models.EpisodeInfo, error) {
	rows, err := db.q.GetEpisodesByMapping(ctx, mappingID)
	if err != nil {
		return nil, err
	}
	episodes := make([]models.EpisodeInfo, 0, len(rows))
	for _, r := range rows {
		episodes = append(episodes, sqlcEpisodeToModel(r))
	}
	return episodes, nil
}

func (db *DB) EpisodeVersion(ctx context.Context, mappingID int64) (string, error) {
	row, err := db.q.GetEpisodeVersion(ctx, mappingID)
	if err != nil {
		return "", err
	}
	monitoredSum := "0"
	if s, ok := row.MonitoredSum.(int64); ok {
		monitoredSum = strconv.FormatInt(s, 10)
	}
	maxScraped := ""
	if s, ok := row.MaxScraped.(string); ok {
		maxScraped = s
	}
	return strconv.FormatInt(row.Count, 10) + "-" + monitoredSum + "-" + maxScraped, nil
}

func (db *DB) UpdateEpisodeMonitor(ctx context.Context, mappingID, sonarrEpisodeID int64, monitored bool) error {
	return db.q.UpdateEpisodeMonitor(ctx, sqlcgen.UpdateEpisodeMonitorParams{
		Monitored:       boolToInt(monitored),
		MappingID:       mappingID,
		SonarrEpisodeID: sonarrEpisodeID,
	})
}

func (db *DB) ReplaceEpisodesForMapping(ctx context.Context, mappingID int64, episodes []models.EpisodeInfo) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := db.q.WithTx(tx)

	if err := qtx.DeleteEpisodesByMapping(ctx, mappingID); err != nil {
		return err
	}

	if len(episodes) == 0 {
		return tx.Commit()
	}

	now := time.Now()
	for _, e := range episodes {
		var absNum *int64
		if e.AbsoluteEpisodeNumber != nil {
			v := int64(*e.AbsoluteEpisodeNumber)
			absNum = &v
		}
		var airDate *time.Time
		if e.AirDate != nil {
			t := time.Time(*e.AirDate)
			airDate = &t
		}
		if err := qtx.InsertEpisode(ctx, sqlcgen.InsertEpisodeParams{
			MappingID:             mappingID,
			EpisodeNum:            int64(e.EpisodeNum),
			Title:                 e.Title,
			Type:                  string(e.Type),
			LastScraped:           now,
			SonarrEpisodeID:       int64(e.SonarrEpisodeID),
			SeasonNumber:          int64(e.SeasonNumber),
			SonarrEpisodeNumber:   int64(e.SonarrEpisodeNumber),
			AbsoluteEpisodeNumber: absNum,
			Monitored:             boolToInt(e.Monitored),
			AirDate:               airDate,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ── Sync logs ──

func (db *DB) CreateSyncLog(ctx context.Context, triggerType string) (int64, error) {
	row, err := db.q.CreateSyncLog(ctx, triggerType)
	if err != nil {
		return 0, err
	}
	// Retention: bound the activity feed by dropping logs past the keep
	// window. Done here so every log-producing path (sync, resync, AFL
	// cache refresh) is covered automatically. Best-effort — a failed
	// prune must not fail the log creation.
	if n, perr := db.q.PruneSyncLogs(ctx, SyncLogRetentionCount); perr != nil {
		log.Warn().Err(perr).Msg("failed to prune old sync logs")
	} else if n > 0 {
		log.Debug().Int64("pruned", n).Msg("pruned old sync logs")
	}
	return row.ID, nil
}

func (db *DB) CompleteSyncLog(ctx context.Context, id int64, status string, showsProcessed, episodesUpdated int, errors string) error {
	if id == 0 {
		return nil
	}
	now := time.Now()
	return db.q.CompleteSyncLog(ctx, sqlcgen.CompleteSyncLogParams{
		CompletedAt:     &now,
		Status:          status,
		ShowsProcessed:  int64(showsProcessed),
		EpisodesUpdated: int64(episodesUpdated),
		Errors:          errors,
		ID:              id,
	})
}

// SyncLogRetentionCount is how many recent sync logs to keep. Older
// rows are pruned automatically as new logs are created.
const SyncLogRetentionCount = 200

// SweepStaleRunningSyncLogs marks logs still in 'running' state as
// failed. Called at startup: any row left running by a previous
// process's shutdown can never complete, and a stuck 'running' row also
// breaks the dashboard's next-sync estimate.
func (db *DB) SweepStaleRunningSyncLogs(ctx context.Context) (int64, error) {
	return db.q.FailStaleRunningSyncLogs(ctx)
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

	qtx := db.q.WithTx(tx)

	if err := qtx.DeleteSyncLogItemsByLog(ctx, logID); err != nil {
		return err
	}

	for _, item := range items {
		if err := qtx.InsertSyncLogItem(ctx, sqlcgen.InsertSyncLogItemParams{
			SyncLogID:       logID,
			MappingID:       item.MappingID,
			SonarrTitle:     item.SonarrTitle,
			Status:          item.Status,
			EpisodesChanged: int64(item.EpisodesChanged),
			Summary:         item.Summary,
			Error:           item.Error,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) GetLatestSyncLog(ctx context.Context) (models.SyncLog, error) {
	row, err := db.q.GetLatestSyncLog(ctx)
	if err != nil {
		return models.SyncLog{}, err
	}
	return sqlcSyncLogToModel(row), nil
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

	rows, err := db.q.GetSyncLogsPage(ctx, sqlcgen.GetSyncLogsPageParams{
		Limit:  int64(limit + 1),
		Offset: int64(offset),
	})
	if err != nil {
		return models.SyncLogPage{}, err
	}

	logs := make([]models.SyncLog, 0, len(rows))
	for _, r := range rows {
		logs = append(logs, sqlcSyncLogToModel(r))
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

	logIDs := make([]int64, len(page.Items))
	for i := range page.Items {
		logIDs[i] = page.Items[i].ID
	}
	itemsByLog, err := db.getSyncLogItemsBatch(ctx, logIDs)
	if err != nil {
		return models.SyncLogPage{}, err
	}
	for i := range page.Items {
		page.Items[i].Items = itemsByLog[page.Items[i].ID]
	}

	return page, nil
}

func (db *DB) GetSyncLogItems(ctx context.Context, logIDs []int64) (map[int64][]models.SyncLogItem, error) {
	return db.getSyncLogItemsBatch(ctx, logIDs)
}

// getSyncLogItemsBatch fetches sync log items for multiple log IDs in a single
// query, avoiding N+1 patterns.
func (db *DB) getSyncLogItemsBatch(ctx context.Context, logIDs []int64) (map[int64][]models.SyncLogItem, error) {
	itemsByLog := make(map[int64][]models.SyncLogItem, len(logIDs))
	if len(logIDs) == 0 {
		return itemsByLog, nil
	}

	placeholders := make([]string, len(logIDs))
	args := make([]any, len(logIDs))
	for i, id := range logIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "SELECT id, sync_log_id, mapping_id, sonarr_title, status, episodes_changed, summary, error " +
		"FROM sync_log_items WHERE sync_log_id IN (" + strings.Join(placeholders, ",") + ") " +
		"ORDER BY sonarr_title COLLATE NOCASE ASC"

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.SyncLogItem
		var syncLogID int64
		var episodesChanged int64
		if err := rows.Scan(
			&item.ID,
			&syncLogID,
			&item.MappingID,
			&item.SonarrTitle,
			&item.Status,
			&episodesChanged,
			&item.Summary,
			&item.Error,
		); err != nil {
			return nil, err
		}
		item.EpisodesChanged = int(episodesChanged)
		itemsByLog[syncLogID] = append(itemsByLog[syncLogID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return itemsByLog, nil
}

// ── Settings ──

func (db *DB) GetSetting(ctx context.Context, key string) (string, error) {
	row, err := db.q.GetSetting(ctx, key)
	if err == sql.ErrNoRows {
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

func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	return db.q.SetSetting(ctx, sqlcgen.SetSettingParams{
		Key:   key,
		Value: value,
	})
}

// ── AFL cache ──

func (db *DB) GetCachedAFLShows(ctx context.Context) ([]models.AFLShow, time.Time, error) {
	rows, err := db.q.GetAllCachedAFLShows(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}

	shows := make([]models.AFLShow, 0, len(rows))
	var refreshedAt time.Time
	for _, r := range rows {
		shows = append(shows, models.AFLShow{
			Title: r.Title,
			Slug:  r.Slug,
			URL:   r.Url,
		})
		if refreshedAt.IsZero() || r.RefreshedAt.After(refreshedAt) {
			refreshedAt = r.RefreshedAt
		}
	}
	return shows, refreshedAt, nil
}

func (db *DB) ReplaceCachedAFLShows(ctx context.Context, shows []models.AFLShow, refreshedAt time.Time) ([]string, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := db.q.WithTx(tx)

	// Collect existing slugs before deleting so we can detect new ones.
	existingSlugs, err := qtx.GetAllCachedAFLSlugs(ctx)
	if err != nil {
		return nil, err
	}
	existingSet := make(map[string]bool, len(existingSlugs))
	for _, s := range existingSlugs {
		existingSet[s] = true
	}

	if err := qtx.DeleteAllCachedAFLShows(ctx); err != nil {
		return nil, err
	}

	var newSlugs []string
	for _, show := range shows {
		if err := qtx.InsertCachedAFLShow(ctx, sqlcgen.InsertCachedAFLShowParams{
			Slug:        show.Slug,
			Title:       show.Title,
			Url:         show.URL,
			RefreshedAt: refreshedAt,
		}); err != nil {
			return nil, err
		}
		if !existingSet[show.Slug] {
			newSlugs = append(newSlugs, show.Slug)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if err := db.SetSetting(ctx, "afl_show_cache_refreshed_at", refreshedAt.UTC().Format(time.RFC3339)); err != nil {
		log.Warn().Err(err).Msg("failed to persist afl_show_cache_refreshed_at")
	}

	return newSlugs, nil
}

func (db *DB) GetAFLShowCacheStatus(ctx context.Context) (int, time.Time, error) {
	count, err := db.q.CountCachedAFLShows(ctx)
	if err != nil {
		return 0, time.Time{}, err
	}

	value, err := db.GetSetting(ctx, "afl_show_cache_refreshed_at")
	if err != nil || value == "" {
		return int(count), time.Time{}, err
	}

	refreshedAt, parseErr := time.Parse(time.RFC3339, value)
	if parseErr != nil {
		return int(count), time.Time{}, nil
	}
	return int(count), refreshedAt, nil
}

// ── Filler stats ──

func (db *DB) GetFillerStats(ctx context.Context) (canon, mixed, animeCanon, filler, total int, err error) {
	row, err := db.q.GetFillerStats(ctx)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	canon = toInt(row.Canon)
	mixed = toInt(row.Mixed)
	animeCanon = toInt(row.AnimeCanon)
	filler = toInt(row.Filler)
	total = int(row.Total)
	return
}

// ── Sonarr cache ──

func (db *DB) GetCachedSonarrSeries(ctx context.Context, seriesID int, maxAge time.Duration) (*models.SonarrSeries, bool) {
	row, err := db.q.GetCachedSonarrSeries(ctx, int64(seriesID))
	if err != nil {
		return nil, false
	}
	if time.Since(row.CachedAt) > maxAge {
		return nil, false
	}
	var series models.SonarrSeries
	if err := json.Unmarshal([]byte(row.DataJson), &series); err != nil {
		return nil, false
	}
	return &series, true
}

func (db *DB) SetCachedSonarrSeries(ctx context.Context, seriesID int, series *models.SonarrSeries) error {
	dataJSON, err := json.Marshal(series)
	if err != nil {
		return err
	}
	return db.q.SetCachedSonarrSeries(ctx, sqlcgen.SetCachedSonarrSeriesParams{
		SeriesID: int64(seriesID),
		DataJson: string(dataJSON),
		CachedAt: time.Now(),
	})
}

// ── Helpers ──

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
