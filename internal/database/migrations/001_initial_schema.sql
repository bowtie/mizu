-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS show_mappings (
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
    artwork_source TEXT NOT NULL DEFAULT '',
    artwork_poster_path TEXT NOT NULL DEFAULT '',
    artwork_banner_path TEXT NOT NULL DEFAULT '',
    artwork_logo_path TEXT NOT NULL DEFAULT '',
    overview TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS episode_info (
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
    air_date DATETIME,
    FOREIGN KEY (mapping_id) REFERENCES show_mappings(id) ON DELETE CASCADE,
    UNIQUE(mapping_id, episode_num)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sync_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    status TEXT NOT NULL DEFAULT 'running',
    shows_processed INTEGER NOT NULL DEFAULT 0,
    episodes_updated INTEGER NOT NULL DEFAULT 0,
    errors TEXT NOT NULL DEFAULT '',
    trigger_type TEXT NOT NULL DEFAULT 'manual'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sync_log_items (
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
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS afl_show_cache (
    slug TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    refreshed_at DATETIME NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sonarr_series_cache (
    series_id INTEGER PRIMARY KEY,
    data_json TEXT NOT NULL,
    cached_at DATETIME NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_episode_info_mapping ON episode_info(mapping_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_episode_info_mapping_num ON episode_info(mapping_id, episode_num);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_episode_info_mapping_sonarr_id ON episode_info(mapping_id, sonarr_episode_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sync_logs_started ON sync_logs(started_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sync_log_items_log_id ON sync_log_items(sync_log_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_afl_show_cache_title ON afl_show_cache(title COLLATE NOCASE);
-- +goose StatementEnd
