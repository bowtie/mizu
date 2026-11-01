package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"mizu/internal/database/sqlcgen"
	"mizu/internal/models"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"
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

// Ping checks that the database is reachable.
func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

func (db *DB) migrate() error {
	goose.SetLogger(goose.NopLogger())

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
