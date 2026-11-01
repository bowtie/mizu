package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"mizu/internal/auth"
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
	logAuthMode(cfg)

	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfg.DBPath).Msg("failed to open database")
	}
	defer db.Close()
	log.Info().Str("path", cfg.DBPath).Msg("database initialized")

	applyRuntimeSettings(context.Background(), db, cfg)
	seedAuthFromEnv(db)
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

	go startScheduler(ctx, cfg, scheduler, syncEngine)
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
	//
	// Mutations take the config write lock: startup calls this before the
	// server is listening, but config import calls it from a request
	// handler where concurrent readers hold the read lock.
	cfg.Lock()
	defer cfg.Unlock()

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
	if value, ok := settings["ntfy_notify_on"]; ok && config.IsValidNtfyNotifyOn(value) {
		cfg.NtfyNotifyOn = value
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

func seedAuthFromEnv(db *database.DB) {
	ctx := context.Background()

	// Drop sessions that expired while the server was down.
	if n, err := db.DeleteExpiredSessions(ctx, time.Now()); err == nil && n > 0 {
		log.Info().Int64("count", n).Msg("removed expired sessions")
	}

	// Mark sync logs stuck in 'running' by a previous shutdown as failed —
	// they can never complete and would skew the next-sync estimate.
	if n, err := db.SweepStaleRunningSyncLogs(ctx); err == nil && n > 0 {
		log.Info().Int64("count", n).Msg("marked interrupted sync logs as failed")
	}

	envPassword := os.Getenv("MIZU_PASSWORD")
	if envPassword == "" {
		return
	}
	mgr := auth.NewManager(db)
	configured, err := mgr.IsConfigured(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to check auth configuration; skipping MIZU_PASSWORD seeding")
		return
	}
	if configured {
		log.Info().Msg("MIZU_PASSWORD set but a password is already configured; ignoring")
		return
	}
	if _, _, err := mgr.Setup(ctx, envPassword); err != nil {
		log.Error().Err(err).Msg("failed to seed password from MIZU_PASSWORD")
		return
	}
	log.Info().Msg("initial dashboard password seeded from MIZU_PASSWORD")
}

func startScheduler(ctx context.Context, cfg *config.Config, scheduler *schedulerController, syncEngine *syncengine.Engine) {
	ticker := time.NewTicker(scheduler.Interval())
	defer ticker.Stop()

	// The ticker only fires a full interval after startup, so a process
	// that restarts more often than that (or sat stopped for days) would
	// never sync. Check once shortly after boot whether a sync is overdue.
	catchUp := time.NewTimer(catchUpSyncDelay)
	defer catchUp.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("scheduler stopped")
			return
		case <-catchUp.C:
			cfg.RLock()
			configured := cfg.SonarrAPIKey != ""
			cfg.RUnlock()
			if !configured {
				continue
			}
			last := lastSyncStartedAt(ctx, syncEngine.DB())
			if !catchUpSyncDue(last, scheduler.Interval(), time.Now()) {
				continue
			}
			log.Info().Msg("overdue sync detected at startup, running catch-up sync")
			if err := syncEngine.RunSync(ctx, "scheduled"); err != nil {
				log.Error().Err(err).Msg("catch-up sync failed")
			}
		case interval := <-scheduler.updates:
			// Reset doesn't drain the channel: a tick that fired just
			// before the interval change would otherwise be delivered
			// immediately, running an unscheduled extra sync.
			select {
			case <-ticker.C:
			default:
			}
			ticker.Reset(interval)
			log.Info().Dur("scan_interval", interval).Msg("scheduler interval updated")
		case <-ticker.C:
			if err := syncEngine.RunSync(ctx, "scheduled"); err != nil {
				log.Error().Err(err).Msg("scheduled sync failed")
			}
		}
	}
}

// logAuthMode records the active authentication mode and warns about the
// combinations that leave the dashboard open to the network.
func logAuthMode(cfg *config.Config) {
	ev := log.Info().Str("auth", string(cfg.Auth.Mode))
	if cfg.Auth.Mode == config.AuthProxy {
		ev = ev.Str("header", cfg.Auth.ProxyHeader).Int("trusted_proxies", len(cfg.Auth.TrustedProxies))
	}
	ev.Msg("authentication mode")
	if cfg.Auth.Mode == config.AuthNone && !isLoopbackAddr(cfg.ListenAddr) {
		log.Warn().Str("listen", cfg.ListenAddr).Msg("MIZU_AUTH=none on a non-loopback address: anyone who can reach this port has full access, including your Sonarr API key. Put it behind an authenticating proxy or VPN.")
	}
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
