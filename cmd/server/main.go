package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"mizu/internal/config"
	"mizu/internal/database"
	syncengine "mizu/internal/sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Mizu %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().Str("version", version).Str("listen", cfg.ListenAddr).Str("sonarr", cfg.SonarrURL).Dur("scan_interval", cfg.ScanInterval).Msg("starting Mizu")

	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfg.DBPath).Msg("failed to open database")
	}
	defer db.Close()
	log.Info().Str("path", cfg.DBPath).Msg("database initialized")

	applyRuntimeSettings(context.Background(), db, cfg)
	scheduler := newSchedulerController(cfg.ScanInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var bgWG sync.WaitGroup
	syncEngine := syncengine.New(cfg, db)
	router := setupRouter(cfg, db, syncEngine, scheduler, ctx, &bgWG)
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go startScheduler(ctx, scheduler, syncEngine)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Info().Str("addr", cfg.ListenAddr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server failed, triggering shutdown")
			quit <- syscall.SIGTERM
		}
	}()

	<-quit

	log.Info().Msg("shutting down...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}
	// Wait for background goroutines (sync, resync, artwork refresh) to finish
	// before closing the database.
	bgWG.Wait()
	log.Info().Msg("Mizu stopped")
}

func applyRuntimeSettings(ctx context.Context, db *database.DB, cfg *config.Config) {
	// Load all settings in a single query instead of one round-trip per key.
	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load runtime settings; using defaults")
		return
	}

	// Keys gated on non-empty values skip when absent, matching the old
	// per-key GetSetting behavior (absent rows returned "" and were skipped).
	// Always-apply keys (API keys, topics) skip when the row is entirely
	// missing, which is strictly safer than the old behavior of clobbering
	// config.Load() defaults with "" on a fresh database.
	if value, ok := settings["sonarr_url"]; ok && value != "" {
		cfg.SonarrURL = strings.TrimRight(value, "/")
	}
	if value, ok := settings["sonarr_api_key"]; ok {
		cfg.SonarrAPIKey = value
	}
	if value, ok := settings["default_filler_action"]; ok && value != "" {
		cfg.DefaultFillerAction = config.FillerAction(value)
	}
	if value, ok := settings["default_mixed_canon_action"]; ok && value != "" {
		cfg.DefaultMixedCanonAction = config.FillerAction(value)
	}
	if value, ok := settings["default_anime_canon_action"]; ok && value != "" {
		cfg.DefaultAnimeCanonAction = config.FillerAction(value)
	}
	if value, ok := settings["sync_interval"]; ok && value != "" {
		if _, d, parseErr := normalizeScanInterval(value); parseErr == nil {
			cfg.ScanInterval = d
		}
	}
	if value, ok := settings["sonarr_add_root_folder"]; ok {
		cfg.SonarrAddRootFolder = value
	}
	if value, ok := settings["sonarr_add_quality_profile_id"]; ok && value != "" {
		if id, parseErr := strconv.Atoi(value); parseErr == nil {
			cfg.SonarrAddQualityProfileID = id
		}
	}
	if value, ok := settings["sonarr_add_monitor_mode"]; ok && value != "" {
		cfg.SonarrAddMonitorMode = value
	}
	if value, ok := settings["sonarr_add_season_folder"]; ok && value != "" {
		cfg.SonarrAddSeasonFolder = value == "true"
	}
	if value, ok := settings["ntfy_enabled"]; ok && value != "" {
		cfg.NtfyEnabled = value == "true"
	}
	if value, ok := settings["ntfy_server_url"]; ok && value != "" {
		cfg.NtfyServerURL = strings.TrimRight(value, "/")
	}
	if value, ok := settings["ntfy_topic"]; ok {
		cfg.NtfyTopic = value
	}
	if value, ok := settings["ntfy_token"]; ok {
		cfg.NtfyToken = value
	}
	if value, ok := settings["artwork_default_source"]; ok && value != "" {
		// Migrate legacy "auto" to "sonarr".
		if value == "auto" {
			value = "sonarr"
			if err := db.SetSetting(ctx, "artwork_default_source", "sonarr"); err != nil {
				log.Warn().Err(err).Msg("failed to migrate artwork_default_source from auto to sonarr")
			}
		}
		cfg.ArtworkDefaultSource = value
	}
	if value, ok := settings["tmdb_api_key"]; ok {
		cfg.TMDBAPIKey = value
	}
	if value, ok := settings["fanart_api_key"]; ok {
		cfg.FanartAPIKey = value
	}
	if value, ok := settings["ui_sounds_enabled"]; ok && value != "" {
		cfg.UISoundsEnabled = value == "true"
	}
}

func startScheduler(ctx context.Context, scheduler *schedulerController, syncEngine *syncengine.Engine) {
	ticker := time.NewTicker(scheduler.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("scheduler stopped")
			return
		case interval := <-scheduler.updates:
			ticker.Reset(interval)
			log.Info().Dur("scan_interval", interval).Msg("scheduler interval updated")
		case <-ticker.C:
			if err := syncEngine.RunSync(ctx, "scheduled"); err != nil {
				log.Error().Err(err).Msg("scheduled sync failed")
			}
		}
	}
}
