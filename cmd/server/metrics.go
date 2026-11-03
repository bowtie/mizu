package main

import (
	"net/http"
	"strconv"
	"time"

	"mizu/internal/auth"
	"mizu/internal/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func registerBuildInfo() {
	metrics.SetLabeledGauge("mizu_build_info", "Build metadata; value is always 1.", 1, map[string]string{
		"version":   version,
		"commit":    commit,
		"buildDate": buildDate,
	})
}

// instrumentRequests counts requests per method/route/status. The route
// pattern (e.g. /api/mappings/{id}) is only known after the handler ran.
func instrumentRequests(next http.Handler) http.Handler {
	metrics.RegisterCounter("mizu_http_requests_total", "HTTP requests processed.", "method", "route", "status")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		metrics.Add("mizu_http_requests_total", r.Method, route, strconv.Itoa(ww.Status()))
	})
}

// aflCacheRefreshMetrics registers the AFL refresh counters once.
func init() {
	metrics.RegisterCounter("mizu_afl_cache_refresh_total", "AFL show-list cache refresh outcomes.", "status")
}

// metricsAuth guards /metrics. Prometheus can't hold a session cookie, so
// besides a signed-in browser it accepts HTTP Basic auth with the
// dashboard password (any username) — configure it as basic_auth in the
// scrape config. Wrong passwords count against the same failure budget as
// the login form. In external sign-in mode the proxy decides, like for
// every other route.
func metricsAuth(mgr *auth.Manager, acc *accessControl, limiter *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := isAuthenticated(r, mgr, acc)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		if ok {
			next.ServeHTTP(w, r)
			return
		}
		if acc.get().external() {
			writeError(w, http.StatusForbidden, "Mizu only accepts requests through your sign-in proxy")
			return
		}
		_, password, hasBasic := r.BasicAuth()
		if !hasBasic {
			w.Header().Set("WWW-Authenticate", `Basic realm="Mizu metrics"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		key := acc.get().clientIP(r).String()
		if allowed, retry := limiter.allow(key, time.Now()); !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry/time.Second)+1))
			writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
			return
		}
		if err := mgr.VerifyCached(r.Context(), password); err != nil {
			limiter.fail(key, time.Now())
			w.Header().Set("WWW-Authenticate", `Basic realm="Mizu metrics"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		limiter.succeed(key)
		next.ServeHTTP(w, r)
	})
}
