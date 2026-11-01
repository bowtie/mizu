package main

import (
	"context"
	"net/http"
	stdsync "sync"
	"time"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/sonarr"
	syncengine "mizu/internal/sync"

	"github.com/rs/zerolog/log"
)

// healthSonarrTTL is how long a Sonarr connectivity result is reused.
// Docker's HEALTHCHECK polls every 30s; without caching each poll would
// fire a test request at Sonarr forever. One in-flight probe is enough.
const healthSonarrTTL = time.Minute

// healthProbeBudget caps a single Sonarr probe (including the client's
// internal retries) so an unresponsive Sonarr can't stretch health
// checks into minutes-long hangs.
const healthProbeBudget = 15 * time.Second

// sonarrStatusCache caches the result of a Sonarr connection test for
// health checks. Checks serialize behind the mutex, so concurrent
// pollers share one probe instead of stampeding Sonarr.
type sonarrStatusCache struct {
	mu        stdsync.Mutex
	healthy   bool
	checkedAt time.Time
	ttl       time.Duration
}

func newSonarrStatusCache(ttl time.Duration) *sonarrStatusCache {
	return &sonarrStatusCache{ttl: ttl}
}

func (c *sonarrStatusCache) healthyWithin(ctx context.Context, client *sonarr.Client) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.checkedAt.IsZero() && time.Since(c.checkedAt) < c.ttl {
		return c.healthy
	}
	probeCtx, cancel := context.WithTimeout(ctx, healthProbeBudget)
	defer cancel()
	_, err := client.TestConnection(probeCtx)
	c.healthy = err == nil
	c.checkedAt = time.Now()
	if err != nil {
		log.Debug().Err(err).Msg("health check: Sonarr unreachable")
	}
	return c.healthy
}

// handleLiveness answers whether Mizu itself is up: the process is serving
// and its database responds. It deliberately ignores Sonarr — a Sonarr
// outage must not mark Mizu unhealthy, or a container supervisor would
// restart it for someone else's problem. This is what the Docker
// HEALTHCHECK probes; /api/health reports the full status, Sonarr included.
func handleLiveness(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down", "error": "database unreachable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func handleHealth(cfg *config.Config, db *database.DB, scheduler *schedulerController, engine *syncengine.Engine, statusCache *sonarrStatusCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		cfg.RLock()
		sonarrURL := cfg.SonarrURL
		cfg.RUnlock()

		degraded := false
		if sonarrURL != "" && !statusCache.healthyWithin(r.Context(), engine.SonarrClient()) {
			status = "degraded"
			degraded = true
		}

		payload := map[string]interface{}{"status": status, "version": version}
		if latest, err := db.GetLatestSyncLog(r.Context()); err == nil {
			payload["last_sync_at"] = latest.CompletedAt
			interval := scheduler.Interval()
			if interval > 0 && latest.CompletedAt != nil {
				next := latest.CompletedAt.Add(interval)
				if next.Before(time.Now()) {
					next = time.Now().Add(interval)
				}
				payload["next_sync_at"] = next
			}
		}

		// 503 on degradation so container orchestrators and uptime
		// monitors can react; previously degraded always returned 200 and
		// the Docker HEALTHCHECK could never fail.
		code := http.StatusOK
		if degraded {
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, payload)
	}
}
