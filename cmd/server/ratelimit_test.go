package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func failN(l *rateLimiter, key string, n int, now time.Time) {
	for i := 0; i < n; i++ {
		l.fail(key, now)
	}
}

func TestRateLimiterLocksKeyAfterFailures(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authPerKeyLimit; i++ {
		ok, _ := l.allow("1.2.3.4", now)
		require.True(t, ok, "attempt %d should be allowed", i+1)
		l.fail("1.2.3.4", now)
	}
	ok, retry := l.allow("1.2.3.4", now)
	assert.False(t, ok)
	assert.Positive(t, retry)
}

func TestRateLimiterSuccessesDontCount(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authPerKeyLimit*3; i++ {
		ok, _ := l.allow("1.2.3.4", now)
		require.True(t, ok)
		l.succeed("1.2.3.4")
	}
}

func TestRateLimiterSuccessClearsFailures(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	failN(l, "1.2.3.4", authPerKeyLimit-1, now)
	l.succeed("1.2.3.4")
	failN(l, "1.2.3.4", authPerKeyLimit-1, now)
	ok, _ := l.allow("1.2.3.4", now)
	assert.True(t, ok)
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	failN(l, "1.2.3.4", authPerKeyLimit, now)
	ok, _ := l.allow("5.6.7.8", now)
	assert.True(t, ok, "a different key must not be denied by another key's window")
}

func TestRateLimiterWindowResets(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	failN(l, "1.2.3.4", authPerKeyLimit, now)
	ok, _ := l.allow("1.2.3.4", now.Add(authWindowPeriod+time.Second))
	assert.True(t, ok, "a fresh window must allow attempts again")
}

func TestRateLimiterGlobalBudgetStopsRotatingAttackers(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authGlobalLimit; i++ {
		l.fail(fmt.Sprintf("10.0.%d.%d", i/250, i%250), now)
	}
	ok, _ := l.allow("10.0.0.1", now)
	assert.False(t, ok, "a source that already failed is held to the exhausted global budget")
}

func TestRateLimiterGlobalBudgetDoesNotLockOutOwner(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authGlobalLimit*2; i++ {
		l.fail(fmt.Sprintf("10.1.%d.%d", i/250, i%250), now)
	}
	ok, _ := l.allow("192.168.1.20", now)
	assert.True(t, ok, "a source with a clean record still gets its attempts")
}

func TestRateLimiterMapIsBounded(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < maxTrackedKeys+500; i++ {
		l.fail(fmt.Sprintf("key-%d", i), now)
	}
	assert.LessOrEqual(t, len(l.perKey), maxTrackedKeys)
	// With the map full, untracked sources fall under the global budget.
	ok, _ := l.allow("brand-new", now)
	assert.False(t, ok)
}

func TestClientIPUsesForwardedForOnlyFromTrustedProxies(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.7")
	none := accessPolicy{}
	lanProxy := accessPolicy{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("192.168.1.5/32")}}

	// A LAN client is not a proxy unless listed: its header is ignored.
	r.RemoteAddr = "192.168.1.5:51000"
	assert.Equal(t, "192.168.1.5", none.clientIP(r).String())
	// Listed: the rightmost entry is the address the proxy saw; anything
	// left of it came from the client and is ignored.
	assert.Equal(t, "203.0.113.7", lanProxy.clientIP(r).String())

	// Same-host proxy over loopback is always trusted.
	r.RemoteAddr = "127.0.0.1:51000"
	assert.Equal(t, "203.0.113.7", none.clientIP(r).String())

	// A direct client can't pick its key.
	r.RemoteAddr = "198.51.100.9:51000"
	assert.Equal(t, "198.51.100.9", lanProxy.clientIP(r).String())

	// Chained trusted proxies are skipped.
	chain := accessPolicy{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	r.RemoteAddr = "10.0.0.2:1"
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 203.0.113.7, 10.0.0.3")
	assert.Equal(t, "203.0.113.7", chain.clientIP(r).String())
}

func TestLimitMiddlewareCountsOnlyFailures(t *testing.T) {
	l := newRateLimiter()
	acc := newAccessControl(accessPolicy{})
	status := http.StatusUnauthorized
	handler := limitMiddleware(l, acc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	send := func(addr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < authPerKeyLimit; i++ {
		assert.Equal(t, http.StatusUnauthorized, send("1.2.3.4:1234").Code)
	}
	rec := send("1.2.3.4:1234")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	secs, err := time.ParseDuration(rec.Header().Get("Retry-After") + "s")
	require.NoError(t, err)
	assert.Positive(t, secs)

	// Spoofing X-Forwarded-For from an untrusted peer doesn't help.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// A different source still passes, and its successes never count.
	status = http.StatusOK
	for i := 0; i < authPerKeyLimit*2; i++ {
		assert.Equal(t, http.StatusOK, send("5.6.7.8:9999").Code)
	}
}
