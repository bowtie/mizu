package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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
	listenAddr := flag.String("addr", "", "listen address (default \":8978\", all interfaces)")
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
	if *listenAddr != "" {
		cfg.ListenAddr = *listenAddr
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
	startupMaintenance(db)
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

	startBackground(ctx, cfg, scheduler, syncEngine, db, &bgWG)
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
	// Wait for background work (scheduled and requested syncs, AFL and
	// artwork refreshes) to finish before closing the database.
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

	// Mutations take the config write lock: startup calls this before the
	// server is listening, but config import calls it from a request
	// handler where concurrent readers hold the read lock. A stored value
	// that no longer parses is skipped, leaving the default.
	cfg.Lock()
	defer cfg.Unlock()
	for _, k := range settingKeys {
		value, ok := settings[k.name]
		if !ok {
			continue
		}
		parsed, err := k.parse(value)
		if err != nil {
			log.Warn().Err(err).Str("setting", k.name).Msg("ignoring stored setting")
			continue
		}
		k.apply(cfg, parsed)
	}
}

// startupMaintenance clears state a previous run left behind.
func startupMaintenance(db *database.DB) {
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
}

// sessionSweepInterval is how often expired sessions are deleted. Request
// handling never deletes them (a bogus cookie must not cost a write), so
// this is the only cleanup besides the one at startup.
const sessionSweepInterval = time.Hour

func sweepSessions(ctx context.Context, mgr *auth.Manager) {
	ticker := time.NewTicker(sessionSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := mgr.SweepExpired(ctx); err != nil {
				log.Warn().Err(err).Msg("failed to sweep expired sessions")
			} else if n > 0 {
				log.Debug().Int64("count", n).Msg("removed expired sessions")
			}
		}
	}
}

// startBackground starts the loops that run until ctx is cancelled. They
// join bgWG so shutdown waits for a sync or refresh they have under way.
func startBackground(ctx context.Context, cfg *config.Config, scheduler *schedulerController, syncEngine *syncengine.Engine, db *database.DB, bgWG *sync.WaitGroup) {
	bgWG.Go(func() { startScheduler(ctx, cfg, scheduler, syncEngine) })
	bgWG.Go(func() { startAFLCacheRefresher(ctx, cfg, syncEngine, db) })
	bgWG.Go(func() { sweepSessions(ctx, auth.NewManager(db)) })
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
