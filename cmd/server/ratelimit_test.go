package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterAllowsUpToPerKeyLimit(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authPerKeyLimit; i++ {
		ok, retry := l.allow("1.2.3.4", now)
		require.True(t, ok, "attempt %d should be allowed", i+1)
		assert.Zero(t, retry)
	}
	ok, retry := l.allow("1.2.3.4", now)
	assert.False(t, ok)
	assert.Positive(t, retry)
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authPerKeyLimit; i++ {
		ok, _ := l.allow("1.2.3.4", now)
		require.True(t, ok)
	}
	ok, _ := l.allow("5.6.7.8", now)
	assert.True(t, ok, "a different key must not be denied by another key's window")
}

func TestRateLimiterWindowResets(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authPerKeyLimit; i++ {
		ok, _ := l.allow("1.2.3.4", now)
		require.True(t, ok)
	}
	after := now.Add(authWindowPeriod + time.Second)
	ok, _ := l.allow("1.2.3.4", after)
	assert.True(t, ok, "a fresh window must allow attempts again")
}

func TestRateLimiterGlobalCapDeniesRotatingKeys(t *testing.T) {
	l := newRateLimiter()
	now := time.Now()
	for i := 0; i < authGlobalLimit; i++ {
		ok, _ := l.allow(fmt.Sprintf("10.0.0.%d", i), now)
		require.True(t, ok, "attempt %d below the global cap should pass", i+1)
	}
	ok, _ := l.allow("fresh-key", now)
	assert.False(t, ok, "the global cap must deny even unseen keys")
}

func TestRequestKeyUsesForwardedForOnlyFromProxies(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)

	// From a proxy on the LAN: the rightmost entry is the address the proxy
	// saw; anything left of it came from the client and is ignored.
	r.RemoteAddr = "192.168.1.5:51000"
	assert.Equal(t, "192.168.1.5", requestKey(r))
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.7")
	assert.Equal(t, "203.0.113.7", requestKey(r))

	// Same-host proxy over loopback.
	r.RemoteAddr = "127.0.0.1:51000"
	assert.Equal(t, "203.0.113.7", requestKey(r))

	// A client connecting directly from the internet can't pick its key.
	r.RemoteAddr = "198.51.100.9:51000"
	assert.Equal(t, "198.51.100.9", requestKey(r))
}

func TestLimitMiddlewareReturns429WithRetryAfter(t *testing.T) {
	l := newRateLimiter()
	handler := limitMiddleware(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	now := time.Now()
	for i := 0; i < authPerKeyLimit; i++ {
		l.allow("1.2.3.4", now)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter := rec.Header().Get("Retry-After")
	assert.NotEmpty(t, retryAfter)
	secs, err := time.ParseDuration(retryAfter + "s")
	require.NoError(t, err)
	assert.Positive(t, secs)

	// A different source still passes through to the handler.
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req2.RemoteAddr = "5.6.7.8:9999"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}
