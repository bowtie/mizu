package main

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// authRateLimits bound online password guessing on every endpoint that
// checks the dashboard password (login, setup, password change, sign-in
// settings, config export with secrets, metrics basic auth).
//
// Only failed attempts count. Per source, a few failures in a window lock
// that source out until the window ends. A shared global failure budget
// bounds the total guess rate from many sources, but it only applies to
// sources that have already failed: an attacker burning the global budget
// can't lock the owner out, because a source with a clean record still
// gets its attempts. A success clears the source's record.
//
// Limits are in-memory and per-process — restarting resets them — which
// is the right tradeoff for a self-hosted dashboard: no configuration, no
// state to persist.
const (
	authPerKeyLimit  = 10  // failures per source per window
	authGlobalLimit  = 200 // failures per process per window
	authWindowPeriod = 5 * time.Minute

	// maxTrackedKeys bounds the per-source map. When it is full of live
	// windows, new sources can't be told apart from clean ones, so the
	// global budget then applies to everyone.
	maxTrackedKeys = 4096
)

// rateWindow is one fixed-window failure counter.
type rateWindow struct {
	start time.Time
	count int
}

func (w *rateWindow) live(now time.Time, period time.Duration) bool {
	return !w.start.IsZero() && now.Sub(w.start) < period
}

func (w *rateWindow) add(now time.Time, period time.Duration) {
	if !w.live(now, period) {
		w.start = now
		w.count = 0
	}
	w.count++
}

type rateLimiter struct {
	mu     sync.Mutex
	perKey map[string]*rateWindow
	global rateWindow
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{perKey: make(map[string]*rateWindow)}
}

// sweepLocked drops expired per-source windows.
func (l *rateLimiter) sweepLocked(now time.Time) {
	for k, w := range l.perKey {
		if !w.live(now, authWindowPeriod) {
			delete(l.perKey, k)
		}
	}
}

// allow reports whether key may make an attempt now, and if not, how long
// until it may.
func (l *rateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	failures := 0
	w, tracked := l.perKey[key]
	if tracked && w.live(now, authWindowPeriod) {
		failures = w.count
	}
	if failures >= authPerKeyLimit {
		return false, w.start.Add(authWindowPeriod).Sub(now)
	}

	full := false
	if !tracked && len(l.perKey) >= maxTrackedKeys {
		l.sweepLocked(now)
		full = len(l.perKey) >= maxTrackedKeys
	}
	if (failures > 0 || full) && l.global.live(now, authWindowPeriod) && l.global.count >= authGlobalLimit {
		return false, l.global.start.Add(authWindowPeriod).Sub(now)
	}
	return true, 0
}

// fail records a failed attempt for key.
func (l *rateLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.global.add(now, authWindowPeriod)
	w, ok := l.perKey[key]
	if !ok {
		if len(l.perKey) >= maxTrackedKeys {
			l.sweepLocked(now)
		}
		if len(l.perKey) >= maxTrackedKeys {
			return // map stays bounded; the global budget covers the rest
		}
		w = &rateWindow{}
		l.perKey[key] = w
	}
	w.add(now, authWindowPeriod)
}

// succeed clears key's failure record.
func (l *rateLimiter) succeed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.perKey, key)
}

// limitMiddleware rejects requests from sources over their failure budget
// (with a Retry-After hint), and records the handler's outcome: a 401 is a
// failed password, a 2xx a success.
func limitMiddleware(l *rateLimiter, acc *accessControl, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := acc.get().clientIP(r).String()
		if ok, retry := l.allow(key, time.Now()); !ok {
			secs := int(retry/time.Second) + 1
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
			return
		}
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		switch status := ww.Status(); {
		case status == http.StatusUnauthorized:
			l.fail(key, time.Now())
		case status >= 200 && status < 300:
			l.succeed(key)
		}
	})
}
