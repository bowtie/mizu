package main

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// authRateLimits bounds online password guessing on /auth/login and
// /auth/setup. Per-key windows slow a single source to a crawl; the
// global window keeps the total guess rate bounded even when an attacker
// rotates spoofed X-Forwarded-For values (each spoofed key gets a fresh
// window, but the shared budget does not grow). Limits are in-memory and
// per-process — restarting resets them — which is the right tradeoff for
// a self-hosted dashboard: no configuration, no state to persist.
const (
	authPerKeyLimit  = 10  // attempts per key per window
	authGlobalLimit  = 200 // attempts per process per window
	authWindowPeriod = 5 * time.Minute

	// maxTrackedKeys bounds the key map: beyond this, expired windows are
	// dropped on the next request rather than growing without limit.
	maxTrackedKeys = 4096
)

// rateWindow is one fixed-window counter.
type rateWindow struct {
	start time.Time
	count int
}

func (w *rateWindow) allow(now time.Time, limit int, period time.Duration) (bool, time.Duration) {
	if now.Sub(w.start) >= period {
		w.start = now
		w.count = 0
	}
	w.count++
	if w.count > limit {
		return false, w.start.Add(period).Sub(now)
	}
	return true, 0
}

// rateLimiter implements a fixed-window limiter with per-key buckets and
// one shared global bucket.
type rateLimiter struct {
	mu     sync.Mutex
	perKey map[string]*rateWindow
	global rateWindow
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{perKey: make(map[string]*rateWindow)}
}

// allow records an attempt for key and reports whether it is within the
// per-key and global limits, returning how long until a retry could
// succeed when denied.
func (l *rateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Opportunistic cleanup so a flood of unique keys can't pin memory.
	if len(l.perKey) >= maxTrackedKeys {
		for k, w := range l.perKey {
			if now.Sub(w.start) >= authWindowPeriod {
				delete(l.perKey, k)
			}
		}
	}

	w, ok := l.perKey[key]
	if !ok {
		w = &rateWindow{}
		l.perKey[key] = w
	}
	ok, retry := w.allow(now, authPerKeyLimit, authWindowPeriod)
	if !ok {
		return false, retry
	}
	ok, retry = l.global.allow(now, authGlobalLimit, authWindowPeriod)
	if !ok {
		return false, retry
	}
	return true, 0
}

// requestKey identifies the request source: the first X-Forwarded-For
// hop when a proxy supplied one (the client as the proxy saw it), else
// the remote address without its port.
func requestKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, found := strings.Cut(xff, ","); found || first != "" {
			if key := strings.TrimSpace(first); key != "" {
				return key
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limitMiddleware returns middleware that denies requests once the
// limiter's budgets are exhausted, with a Retry-After hint.
func limitMiddleware(l *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, retry := l.allow(requestKey(r), time.Now()); !ok {
			secs := int(retry/time.Second) + 1
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeError(w, http.StatusTooManyRequests, "too many attempts — try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
