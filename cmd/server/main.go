package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/config"
	"github.com/animefillerguard/animefillerguard/internal/database"
	"github.com/animefillerguard/animefillerguard/internal/sync"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		fmt.Printf("AnimeFillerGuard %s (commit: %s, built: %s)\n", version, commit, buildDate)
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

	log.Info().Str("version", version).Str("listen", cfg.ListenAddr).Str("sonarr", cfg.SonarrURL).Dur("scan_interval", cfg.ScanInterval).Msg("starting AnimeFillerGuard")

	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfg.DBPath).Msg("failed to open database")
	}
	defer db.Close()
	log.Info().Str("path", cfg.DBPath).Msg("database initialized")

	applyRuntimeSettings(context.Background(), db, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	syncEngine := sync.New(cfg, db)
	router := setupRouter(cfg, db, syncEngine)
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go startScheduler(ctx, cfg, syncEngine)
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
	log.Info().Msg("AnimeFillerGuard stopped")
}

func applyRuntimeSettings(ctx context.Context, db *database.DB, cfg *config.Config) {
	if value, err := db.GetSetting(ctx, "sonarr_url"); err == nil && value != "" {
		cfg.SonarrURL = value
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
		if d, parseErr := time.ParseDuration(value); parseErr == nil {
			cfg.ScanInterval = d
		}
	}
	if value, err := db.GetSetting(ctx, "enable_tagging"); err == nil && value != "" {
		cfg.EnableTagging = value == "true"
	}
	if value, err := db.GetSetting(ctx, "filler_tag_name"); err == nil && value != "" {
		cfg.FillerTagName = value
	}
}

func startScheduler(ctx context.Context, cfg *config.Config, syncEngine *sync.Engine) {
	ticker := time.NewTicker(cfg.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("scheduler stopped")
			return
		case <-ticker.C:
			if err := syncEngine.RunSync(ctx, "scheduled"); err != nil {
				log.Error().Err(err).Msg("scheduled sync failed")
			}
		}
	}
}
