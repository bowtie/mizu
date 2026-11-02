package config

import (
	"fmt"
	"sync"
	"time"
)

// Config holds all application configuration.
// Runtime values are loaded from SQLite (app_settings table) and overlaid
// onto DefaultConfig() at startup. Mizu reads no environment variables:
// everything is configured in the web UI and stored in the database.
// The RWMutex protects fields from concurrent reads (HTTP handlers) and
// writes (handleUpdateSettings). Callers must hold RLock before reading
// and Lock before writing when running in a concurrent context.
type Config struct {
	mu sync.RWMutex

	// Sonarr connection
	SonarrURL    string
	SonarrAPIKey string

	// Scheduler
	ScanInterval time.Duration

	// Server
	ListenAddr string
	BaseURL    string

	// Database
	DBPath string

	// Scraper
	CacheTTL       time.Duration
	RequestTimeout time.Duration
	RateLimitDelay time.Duration

	// Defaults
	DefaultFillerAction       FillerAction
	DefaultMixedCanonAction   FillerAction
	DefaultAnimeCanonAction   FillerAction
	SonarrAddRootFolder       string
	SonarrAddQualityProfileID int
	SonarrAddMonitorMode      string
	SonarrAddSeasonFolder     bool

	// Notifications
	NtfyEnabled   bool
	NtfyServerURL string
	NtfyTopic     string
	NtfyToken     string
	// NtfyNotifyOn controls which sync results trigger a notification:
	// "always" (default), "changes" (episodes updated or any failure),
	// or "failures".
	NtfyNotifyOn string
	// NtfyNotifyNewTitles sends a push when an AnimeFillerList refresh
	// finds shows that were not in the cache before. Off by default.
	NtfyNotifyNewTitles bool

	// Artwork
	ArtworkDefaultSource string // "sonarr" (default), "anilist", "tmdb", "fanart", "kitsu"
	TMDBAPIKey           string
	FanartAPIKey         string

	// Logging
	LogLevel string

	// Interface
	UISoundsEnabled bool
}

// RLock acquires a read lock on the config. Callers must call RUnlock when done.
func (c *Config) RLock()   { c.mu.RLock() }
func (c *Config) RUnlock() { c.mu.RUnlock() }

// Lock acquires a write lock on the config. Callers must call Unlock when done.
func (c *Config) Lock()   { c.mu.Lock() }
func (c *Config) Unlock() { c.mu.Unlock() }

// FillerAction defines what to do with filler episodes.
type FillerAction string

const (
	ActionUnmonitor FillerAction = "unmonitor"
	ActionMonitor   FillerAction = "monitor"
)

// DefaultConfig returns configuration with sensible defaults.
// These are overridden by values stored in the SQLite app_settings table.
func DefaultConfig() *Config {
	return &Config{
		SonarrURL:                 "http://localhost:8989",
		SonarrAPIKey:              "",
		ScanInterval:              6 * time.Hour,
		ListenAddr:                ":8978",
		BaseURL:                   "/",
		DBPath:                    "mizu.db",
		CacheTTL:                  24 * time.Hour,
		RequestTimeout:            30 * time.Second,
		RateLimitDelay:            2 * time.Second,
		DefaultFillerAction:       ActionUnmonitor,
		DefaultMixedCanonAction:   ActionMonitor,
		DefaultAnimeCanonAction:   ActionMonitor,
		SonarrAddRootFolder:       "",
		SonarrAddQualityProfileID: 0,
		SonarrAddMonitorMode:      "all",
		SonarrAddSeasonFolder:     true,
		NtfyEnabled:               false,
		NtfyServerURL:             "https://ntfy.sh",
		NtfyTopic:                 "",
		NtfyToken:                 "",
		NtfyNotifyOn:              "always",
		ArtworkDefaultSource:      "sonarr",
		TMDBAPIKey:                "",
		FanartAPIKey:              "",
		LogLevel:                  "info",
		UISoundsEnabled:           true,
	}
}

// Load returns the default configuration, validated. Runtime values are
// overlaid from the SQLite database by the caller (see
// applyRuntimeSettings in cmd/server/main.go).
//
// The default listen address is every interface on port 8978: inside a
// container a loopback bind would be unreachable, and a password is
// required before anything but first-run setup is served.
func Load() (*Config, error) {
	cfg := DefaultConfig()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.SonarrURL == "" {
		return fmt.Errorf("sonarr_url is required")
	}
	if c.ScanInterval < 1*time.Minute {
		return fmt.Errorf("scan_interval must be at least 1 minute")
	}
	if c.CacheTTL < 1*time.Hour {
		return fmt.Errorf("cache_ttl must be at least 1 hour")
	}
	switch c.DefaultFillerAction {
	case ActionUnmonitor, ActionMonitor:
		// valid
	default:
		return fmt.Errorf("invalid default_filler_action: %s", c.DefaultFillerAction)
	}
	switch c.DefaultMixedCanonAction {
	case ActionUnmonitor, ActionMonitor:
		// valid
	default:
		return fmt.Errorf("invalid default_mixed_canon_action: %s", c.DefaultMixedCanonAction)
	}
	switch c.DefaultAnimeCanonAction {
	case ActionUnmonitor, ActionMonitor:
		// valid
	default:
		return fmt.Errorf("invalid default_anime_canon_action: %s", c.DefaultAnimeCanonAction)
	}
	if !IsValidSonarrMonitorMode(c.SonarrAddMonitorMode) {
		return fmt.Errorf("invalid sonarr_add_monitor_mode: %s", c.SonarrAddMonitorMode)
	}
	if !IsValidNtfyNotifyOn(c.NtfyNotifyOn) {
		return fmt.Errorf("invalid ntfy_notify_on: %s", c.NtfyNotifyOn)
	}
	return nil
}

// IsValidNtfyNotifyOn returns true if the value is a valid notification
// filter mode.
func IsValidNtfyNotifyOn(value string) bool {
	switch value {
	case "always", "changes", "failures":
		return true
	default:
		return false
	}
}

// IsValidSonarrMonitorMode returns true if the value is a valid Sonarr monitor mode.
func IsValidSonarrMonitorMode(value string) bool {
	switch value {
	case "", "all", "future", "missing", "existing", "pilot", "firstSeason", "latestSeason", "none":
		return true
	default:
		return false
	}
}

// IsValidFillerAction returns true if the value is a valid filler action.
func IsValidFillerAction(value string) bool {
	switch FillerAction(value) {
	case ActionUnmonitor, ActionMonitor:
		return true
	default:
		return false
	}
}
