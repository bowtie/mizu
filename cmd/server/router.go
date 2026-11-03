package main

import (
	"context"
	"net/http"
	"path"
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
	authMgr := auth.NewManager(db)
	acc := newAccessControl(loadAccessPolicy(appCtx, db))

	r := chi.NewRouter()
	r.Use(securityHeaders(acc))
	r.Use(hostGuard(acc))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(instrumentRequests)
	r.Use(limitRequestBody)

	registerBuildInfo()

	// Artwork manager — "sonarr" (default) uses Sonarr images directly;
	// other providers download artwork locally for offline browsing.
	artworkMgr := artwork.NewManager(filepath.Join(filepath.Dir(cfg.DBPath), "artwork"), db)
	artworkMgr.Register(artwork.NewAniListProvider(cfg.RequestTimeout))
	artworkMgr.Register(artwork.NewTMDBProvider(cfg.TMDBAPIKey, cfg.RequestTimeout))
	artworkMgr.Register(artwork.NewFanartProvider(cfg.FanartAPIKey, cfg.RequestTimeout))
	artworkMgr.Register(artwork.NewKitsuProvider(cfg.RequestTimeout))

	// One failure budget for every endpoint that checks the dashboard
	// password, so guesses can't be spread across them.
	authLimiter := newRateLimiter()
	wrapLimited := func(h http.HandlerFunc) http.HandlerFunc {
		return limitMiddleware(authLimiter, acc, h).ServeHTTP
	}

	// Prometheus-style metrics: signed-in browsers, or Basic auth with the
	// dashboard password for scrapers (see metricsAuth).
	r.Handle("/metrics", metricsAuth(authMgr, acc, authLimiter, metrics.Handler()))

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
		r.Get("/health", handleHealth(cfg, db, scheduler, syncEngine, sonarrStatus, authMgr, acc))
		r.Get("/health/live", handleLiveness(db))

		r.Route("/auth", func(r chi.Router) {
			r.Get("/status", handleAuthStatus(authMgr, acc))
			r.Post("/setup", wrapLimited(handleAuthSetup(authMgr, acc, db)))
			r.Post("/login", wrapLimited(handleAuthLogin(authMgr, acc)))
			r.Post("/logout", handleAuthLogout(authMgr, acc))
		})

		// Sonarr's webhook carries its own token (see handleSonarrWebhook).
		r.Post("/webhook/sonarr", handleSonarrWebhook(db, syncEngine, appCtx, bgWG))

		// ── Protected: everything below requires sign-in ──
		r.Group(func(r chi.Router) {
			r.Use(requireAuth(authMgr, acc))

			r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, map[string]interface{}{"version": version, "commit": commit, "build_date": buildDate})
			})

			r.Post("/auth/change-password", wrapLimited(handleAuthChangePassword(authMgr, acc)))
			r.Get("/auth/access", handleGetAccessSettings(authMgr, acc))
			r.Put("/auth/access", wrapLimited(handleUpdateAccessSettings(authMgr, acc, db)))

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
					r.Post("/artwork/custom/{slot}", handleUploadCustomArtwork(db, artworkMgr))
					r.Delete("/artwork/custom/{slot}", handleRemoveCustomArtwork(db, artworkMgr))
					r.Get("/episodes", handleGetEpisodes(db, syncEngine))
					r.Get("/episodes/sonarr", handleGetSonarrEpisodes(db, syncEngine))
					r.Put("/episodes/{epId}/monitor", handleSetEpisodeMonitor(db, syncEngine))
					r.Post("/resync", handleResyncMapping(syncEngine, appCtx, bgWG))
					r.Get("/filler-files", handleGetFillerFiles(syncEngine))
					r.Post("/filler-files/delete", handleDeleteFillerFiles(syncEngine))
				})
			})

			r.Post("/sync", handleTriggerSync(syncEngine, appCtx, bgWG))
			r.Get("/logs", handleGetLogs(db))
			r.Get("/settings", handleGetSettings(cfg, db))
			r.Put("/settings", handleUpdateSettings(cfg, db, scheduler, syncEngine, artworkMgr))
			r.Post("/settings/afl-cache/refresh", handleRefreshAFLCache(syncEngine, db, appCtx, bgWG))
			r.Get("/settings/afl-cache/status", handleAFLCacheRefreshStatus)
			r.Get("/settings/webhook", handleGetWebhook(db))
			r.Post("/settings/webhook/regenerate", handleRegenerateWebhook(db))
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

			// Config backup. A plain export leaves secrets out; including
			// them takes the dashboard password (see handleExportConfig).
			r.Get("/backup/export", handleExportConfig(db, authMgr, acc))
			r.Post("/backup/export", wrapLimited(handleExportConfig(db, authMgr, acc)))
			r.Post("/backup/import", handleImportConfig(cfg, db, scheduler, syncEngine, artworkMgr))
		})
	})

	r.NotFound(web.SPAHandler().ServeHTTP)
	return r
}

// securityHeaders sets browser hardening headers on every response:
// no framing (clickjacking), no MIME sniffing, a strict Content Security
// Policy for the app, and HSTS once the browser is on HTTPS. Handlers may
// override (artwork is served with its own sandbox CSP).
func securityHeaders(acc *accessControl) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", web.CSP)
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			if acc.get().isHTTPS(r) {
				h.Set("Strict-Transport-Security", "max-age=31536000")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// defaultMaxRequestBody caps request bodies: every JSON handler decodes
// without an individual limit, so an oversized body could balloon memory.
// 1 MiB is far above the largest legitimate JSON payload (settings,
// add-series).
const defaultMaxRequestBody = 1 << 20

// customArtworkUploadPath matches the custom artwork upload route.
const customArtworkUploadPath = "/api/mappings/*/artwork/custom/*"

// limitRequestBody wraps every request body in a MaxBytesReader. Custom
// artwork uploads get the larger upload cap instead of the default one:
// a MaxBytesReader can't be loosened by an inner one, so the handler's own
// limit would otherwise never be reachable.
func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody(r))
		}
		next.ServeHTTP(w, r)
	})
}

func maxRequestBody(r *http.Request) int64 {
	if r.Method == http.MethodPost {
		if ok, _ := path.Match(customArtworkUploadPath, r.URL.Path); ok {
			return maxCustomArtworkUploadBody
		}
	}
	return defaultMaxRequestBody
}
