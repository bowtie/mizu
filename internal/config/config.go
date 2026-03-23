package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	// Sonarr connection
	SonarrURL    string `yaml:"sonarr_url"`
	SonarrAPIKey string `yaml:"sonarr_api_key"`

	// Scheduler
	ScanInterval time.Duration `yaml:"scan_interval"`

	// Server
	ListenAddr string `yaml:"listen_addr"`
	BaseURL    string `yaml:"base_url"`

	// Database
	DBPath string `yaml:"db_path"`

	// Scraper
	CacheTTL       time.Duration `yaml:"cache_ttl"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	RateLimitDelay time.Duration `yaml:"rate_limit_delay"`

	// Defaults
	DefaultFillerAction     FillerAction `yaml:"default_filler_action"`
	DefaultMixedCanonAction FillerAction `yaml:"default_mixed_canon_action"`
	DefaultAnimeCanonAction FillerAction `yaml:"default_anime_canon_action"`
	EnableTagging           bool         `yaml:"enable_tagging"`
	FillerTagName           string       `yaml:"filler_tag_name"`

	// Notifications
	NtfyEnabled   bool   `yaml:"ntfy_enabled"`
	NtfyServerURL string `yaml:"ntfy_server_url"`
	NtfyTopic     string `yaml:"ntfy_topic"`
	NtfyToken     string `yaml:"ntfy_token"`

	// Logging
	LogLevel string `yaml:"log_level"`
}

// FillerAction defines what to do with filler episodes.
type FillerAction string

const (
	ActionUnmonitor FillerAction = "unmonitor"
	ActionMonitor   FillerAction = "monitor"
	ActionSkip      FillerAction = "skip" // unmonitor + delete if exists
)

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		SonarrURL:               "http://localhost:8989",
		SonarrAPIKey:            "",
		ScanInterval:            6 * time.Hour,
		ListenAddr:              ":8978",
		BaseURL:                 "/",
		DBPath:                  "/config/mizu.db",
		CacheTTL:                24 * time.Hour,
		RequestTimeout:          30 * time.Second,
		RateLimitDelay:          2 * time.Second,
		DefaultFillerAction:     ActionUnmonitor,
		DefaultMixedCanonAction: ActionMonitor,
		DefaultAnimeCanonAction: ActionMonitor,
		EnableTagging:           false,
		FillerTagName:           "filler",
		NtfyEnabled:             false,
		NtfyServerURL:           "https://ntfy.sh",
		NtfyTopic:               "",
		NtfyToken:               "",
		LogLevel:                "info",
	}
}

// Load reads configuration from a YAML file, then overlays environment variables.
// Environment variables take precedence over the YAML file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try loading YAML config file
	if path != "" {
		if err := cfg.loadYAML(path); err != nil {
			// Only error if file was explicitly specified and doesn't exist
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("loading config file: %w", err)
			}
		}
	}

	// Overlay environment variables
	cfg.loadEnv()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func (c *Config) loadYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

func (c *Config) loadEnv() {
	if v := os.Getenv("SONARR_URL"); v != "" {
		c.SonarrURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("SONARR_API_KEY"); v != "" {
		c.SonarrAPIKey = v
	}
	if v := os.Getenv("SCAN_INTERVAL"); v != "" {
		if d, err := parseDuration(v); err == nil {
			c.ScanInterval = d
		}
	}
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		c.ListenAddr = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if d, err := parseDuration(v); err == nil {
			c.CacheTTL = d
		}
	}
	if v := os.Getenv("DEFAULT_FILLER_ACTION"); v != "" {
		c.DefaultFillerAction = FillerAction(v)
	}
	if v := os.Getenv("DEFAULT_MIXED_CANON_ACTION"); v != "" {
		c.DefaultMixedCanonAction = FillerAction(v)
	}
	if v := os.Getenv("DEFAULT_ANIME_CANON_ACTION"); v != "" {
		c.DefaultAnimeCanonAction = FillerAction(v)
	}
	if v := os.Getenv("ENABLE_TAGGING"); v != "" {
		c.EnableTagging = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("FILLER_TAG_NAME"); v != "" {
		c.FillerTagName = v
	}
	if v := os.Getenv("NTFY_ENABLED"); v != "" {
		c.NtfyEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("NTFY_SERVER_URL"); v != "" {
		c.NtfyServerURL = v
	}
	if v := os.Getenv("NTFY_TOPIC"); v != "" {
		c.NtfyTopic = v
	}
	if v := os.Getenv("NTFY_TOKEN"); v != "" {
		c.NtfyToken = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
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
	case ActionUnmonitor, ActionMonitor, ActionSkip:
		// valid
	default:
		return fmt.Errorf("invalid default_filler_action: %s", c.DefaultFillerAction)
	}
	switch c.DefaultMixedCanonAction {
	case ActionUnmonitor, ActionMonitor, ActionSkip:
		// valid
	default:
		return fmt.Errorf("invalid default_mixed_canon_action: %s", c.DefaultMixedCanonAction)
	}
	switch c.DefaultAnimeCanonAction {
	case ActionUnmonitor, ActionMonitor, ActionSkip:
		// valid
	default:
		return fmt.Errorf("invalid default_anime_canon_action: %s", c.DefaultAnimeCanonAction)
	}
	return nil
}

// parseDuration handles both Go duration strings ("6h") and plain integers ("360" = minutes).
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if mins, err := strconv.Atoi(s); err == nil {
		return time.Duration(mins) * time.Minute, nil
	}
	return 0, fmt.Errorf("invalid duration: %s", s)
}
