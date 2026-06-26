package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"mizu/internal/config"
	"mizu/internal/database"
	syncengine "mizu/internal/sync"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	configPath := flag.String("config", "", "path to config YAML file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Mizu %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load(*configPath)
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

	syncEngine := syncengine.New(cfg, db)
	router := setupRouter(cfg, db, syncEngine, scheduler)
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go startScheduler(ctx, scheduler, syncEngine)
	go func() {
		log.Info().Str("addr", cfg.ListenAddr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}
	log.Info().Msg("Mizu stopped")
}

func applyRuntimeSettings(ctx context.Context, db *database.DB, cfg *config.Config) {
	if value, err := db.GetSetting(ctx, "sonarr_url"); err == nil && value != "" {
		cfg.SonarrURL = value
	}
	if value, err := db.GetSetting(ctx, "sonarr_api_key"); err == nil {
		cfg.SonarrAPIKey = value
	}
	if value, err := db.GetSetting(ctx, "default_filler_action"); err == nil && value != "" {
		cfg.DefaultFillerAction = config.FillerAction(value)
	}
	if value, err := db.GetSetting(ctx, "default_mixed_canon_action"); err == nil && value != "" {
		cfg.DefaultMixedCanonAction = config.FillerAction(value)
	}
	if value, err := db.GetSetting(ctx, "default_anime_canon_action"); err == nil && value != "" {
		cfg.DefaultAnimeCanonAction = config.FillerAction(value)
	}
	if value, err := db.GetSetting(ctx, "sync_interval"); err == nil && value != "" {
		if _, d, parseErr := normalizeScanInterval(value); parseErr == nil {
			cfg.ScanInterval = d
		}
	}
	if value, err := db.GetSetting(ctx, "enable_tagging"); err == nil && value != "" {
		cfg.EnableTagging = value == "true"
	}
	if value, err := db.GetSetting(ctx, "filler_tag_name"); err == nil && value != "" {
		cfg.FillerTagName = value
	}
	if value, err := db.GetSetting(ctx, "enable_anilist_artwork"); err == nil && value != "" {
		cfg.EnableAniListArtwork = value == "true"
	}
	if value, err := db.GetSetting(ctx, "sonarr_add_root_folder"); err == nil {
		cfg.SonarrAddRootFolder = value
	}
	if value, err := db.GetSetting(ctx, "sonarr_add_quality_profile_id"); err == nil && value != "" {
		if id, parseErr := strconv.Atoi(value); parseErr == nil {
			cfg.SonarrAddQualityProfileID = id
		}
	}
	if value, err := db.GetSetting(ctx, "sonarr_add_monitor_mode"); err == nil && value != "" {
		cfg.SonarrAddMonitorMode = value
	}
	if value, err := db.GetSetting(ctx, "sonarr_add_season_folder"); err == nil && value != "" {
		cfg.SonarrAddSeasonFolder = value == "true"
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
