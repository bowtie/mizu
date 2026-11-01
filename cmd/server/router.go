package main

import (
	"context"
	"net/http"
	"path/filepath"
	stdsync "sync"

	"mizu/internal/artwork"
	"mizu/internal/auth"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/metrics"
	"mizu/internal/sync"
	"mizu/internal/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func setupRouter(cfg *config.Config, db *database.DB, syncEngine *sync.Engine, scheduler *schedulerController, appCtx context.Context, bgWG *stdsync.WaitGroup) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(instrumentRequests)
	// Cap request bodies: every handler decodes JSON without an individual
	// limit, so an oversized body could balloon memory. 1 MiB is far above
	// the largest legitimate payload (settings, add-series).
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			}
			next.ServeHTTP(w, r)
		})
	})

	authMgr := auth.NewManager(db)
	registerBuildInfo()

	// Artwork manager — "sonarr" (default) uses Sonarr images directly;
	// other providers download artwork locally for offline browsing.
	artworkMgr := artwork.NewManager(filepath.Join(filepath.Dir(cfg.DBPath), "artwork"), db)
	artworkMgr.Register(artwork.NewAniListProvider(cfg.RequestTimeout))
	artworkMgr.Register(artwork.NewTMDBProvider(cfg.TMDBAPIKey, cfg.RequestTimeout))
	artworkMgr.Register(artwork.NewFanartProvider(cfg.FanartAPIKey, cfg.RequestTimeout))
	artworkMgr.Register(artwork.NewKitsuProvider(cfg.RequestTimeout))

	// Prometheus-style metrics. Public by convention; exposes only
	// counters/gauges, never request payloads.
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api", func(r chi.Router) {
		// Unknown API paths get a JSON 404 instead of falling through to
		// the SPA's index.html (which would answer 200 with HTML).
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not found")
		})

		// CSRF/DNS-rebinding guard for cookie-authed mutations.
		r.Use(sameOriginGuard)

		// ── Public: monitoring + authentication ──
		sonarrStatus := newSonarrStatusCache(healthSonarrTTL)
		r.Get("/health", handleHealth(cfg, db, scheduler, syncEngine, sonarrStatus))
		r.Get("/health/live", handleLiveness(db))

		r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"version": version, "commit": commit, "build_date": buildDate})
		})

		r.Route("/auth", func(r chi.Router) {
			r.Get("/status", handleAuthStatus(authMgr))
			// Login and setup are the password-guessing surfaces — bound
			// by a per-source and a global fixed window so neither a
			// single client nor a key-rotating one can brute-force freely.
			authLimiter := newRateLimiter()
			wrapLimited := func(h http.HandlerFunc) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					limitMiddleware(authLimiter, h).ServeHTTP(w, r)
				}
			}
			r.Post("/setup", wrapLimited(handleAuthSetup(authMgr)))
			r.Post("/login", wrapLimited(handleAuthLogin(authMgr)))
			r.Post("/logout", handleAuthLogout(authMgr))
		})

		// ── Protected: everything below requires a valid session ──
		r.Group(func(r chi.Router) {
			r.Use(requireAuth(authMgr))

			r.Post("/auth/change-password", handleAuthChangePassword(authMgr))
			r.Post("/auth/remove-password", handleAuthRemovePassword(authMgr))

			r.Get("/stats", handleGetStats(db))

			// Artwork — serve locally stored images (works offline)
			r.Get("/artwork/providers", handleGetArtworkProviders(artworkMgr))
			r.Get("/artwork/{mappingId}/poster", handleServeArtwork(artworkMgr, "poster"))
			r.Get("/artwork/{mappingId}/banner", handleServeArtwork(artworkMgr, "banner"))
			r.Get("/artwork/{mappingId}/logo", handleServeArtwork(artworkMgr, "logo"))
			r.Post("/artwork/refresh-all", handleRefreshAllArtwork(cfg, db, artworkMgr, syncEngine, appCtx, bgWG))
			r.Get("/artwork/refresh-status", handleArtworkRefreshStatus)

			r.Route("/mappings", func(r chi.Router) {
				r.Get("/", handleGetMappings(db))
				r.Post("/", handleCreateMapping(cfg, db, syncEngine, artworkMgr, appCtx, bgWG))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", handleGetMapping(db))
					r.Put("/", handleUpdateMapping(syncEngine, db))
					r.Delete("/", handleDeleteMapping(db, artworkMgr))
					r.Post("/artwork/refresh", handleRefreshArtwork(cfg, db, artworkMgr, syncEngine))
					r.Put("/artwork/source", handleSetArtworkSource(db, artworkMgr))
					r.Get("/artwork/candidates", handleListArtworkCandidates(cfg, db, artworkMgr, syncEngine))
					r.Put("/artwork/select", handleSelectArtwork(cfg, db, artworkMgr, syncEngine))
					r.Get("/episodes", handleGetEpisodes(db, syncEngine))
					r.Get("/episodes/sonarr", handleGetSonarrEpisodes(db, syncEngine))
					r.Put("/episodes/{epId}/monitor", handleSetEpisodeMonitor(db, syncEngine))
					r.Post("/resync", handleResyncMapping(syncEngine, appCtx, bgWG))
				})
			})

			r.Post("/sync", handleTriggerSync(syncEngine, appCtx, bgWG))
			r.Get("/logs", handleGetLogs(db))
			r.Get("/settings", handleGetSettings(cfg, db))
			r.Put("/settings", handleUpdateSettings(cfg, db, scheduler, syncEngine, artworkMgr))
			r.Post("/settings/afl-cache/refresh", handleRefreshAFLCache(syncEngine, db, appCtx, bgWG))
			r.Get("/settings/afl-cache/status", handleAFLCacheRefreshStatus)
			r.Get("/sonarr/series", handleGetSonarrSeries(db, syncEngine))
			r.Get("/sonarr/series/{id}", handleGetSonarrSeriesByID(db, syncEngine))
			r.Post("/sonarr/series", handleAddSonarrSeries(cfg, syncEngine))
			r.Get("/sonarr/lookup", handleLookupSonarrSeries(syncEngine))
			r.Get("/sonarr/add-options", handleGetSonarrAddOptions(syncEngine))
			r.Post("/sonarr/manual-match-pending", handleMarkManualMatchPending(db))
			r.Post("/sonarr/test", handleTestSonarr(cfg))
			r.Post("/ntfy/test", handleTestNtfy(cfg))
			r.Get("/afl/search", handleAFLSearch(syncEngine, db))
			r.Get("/afl/suggest", handleAFLSuggest(syncEngine, db))

			// Config backup — session-protected like the settings endpoints
			// it mirrors (the export includes API keys by design).
			r.Get("/backup/export", handleExportConfig(db))
			r.Post("/backup/import", handleImportConfig(cfg, db, scheduler, syncEngine, artworkMgr))
		})
	})

	r.NotFound(web.SPAHandler().ServeHTTP)
	return r
}
