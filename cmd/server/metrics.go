package main

import (
	"net/http"
	"strconv"

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
