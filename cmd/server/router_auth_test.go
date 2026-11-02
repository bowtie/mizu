package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/auth"
	"mizu/internal/config"
	"mizu/internal/database"
	syncengine "mizu/internal/sync"
)

// newAuthTestRouter builds a router backed by a temp database with
// Sonarr connectivity disabled so no handler reaches out over the network.
func newAuthTestRouter(t *testing.T) (http.Handler, *database.DB) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	cfg := config.DefaultConfig()
	cfg.SonarrURL = ""
	cfg.DBPath = filepath.Join(t.TempDir(), "cfg.db")
	syncEngine := syncengine.New(cfg, db)
	router := setupRouter(cfg, db, syncEngine, newSchedulerController(cfg.ScanInterval), t.Context(), nil)
	return router, db
}

func doJSON(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mizu_session" && c.Value != "" {
			return c
		}
	}
	t.Fatal("no session cookie in response")
	return nil
}

// signIn completes first-run setup and returns the session cookie.
func signIn(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"hunter2hunter2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return sessionCookie(t, rec)
}

type authStatus struct {
	Configured    bool `json:"configured"`
	Authenticated bool `json:"authenticated"`
	External      bool `json:"external"`
}

func getStatus(t *testing.T, handler http.Handler, remote string) authStatus {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	if remote != "" {
		req.RemoteAddr = remote
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var st authStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	return st
}

// A fresh install serves nothing but first-run setup: no settings, no
// backup export, no library, until a password is set.
func TestUnconfiguredInstanceIsLocked(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	for _, path := range []string{"/api/mappings", "/api/settings", "/api/backup/export", "/api/version"} {
		rec := doJSON(t, handler, http.MethodGet, path, "", nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
	rec := doJSON(t, handler, http.MethodPut, "/api/settings", `{"sonarr_url":"http://attacker:1"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	assert.Equal(t, authStatus{}, getStatus(t, handler, ""))

	cookie := signIn(t, handler)
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", cookie)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSetupLoginAndProtectedRoutes(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	// Setup issues a session and configures auth.
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"hunter2hunter2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cookie := sessionCookie(t, rec)
	assert.True(t, cookie.HttpOnly)

	// Second setup is rejected.
	rec = doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"another-pass1"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// Protected route works with the cookie.
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", cookie)
	assert.Equal(t, http.StatusOK, rec.Code)

	// ...and returns 401 without it.
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Health stays public for monitoring but says only up/down; the
	// version needs sign-in.
	rec = doJSON(t, handler, http.MethodGet, "/api/health", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "version")
	rec = doJSON(t, handler, http.MethodGet, "/api/version", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = doJSON(t, handler, http.MethodGet, "/api/version", "", cookie)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Logout revokes the session.
	rec = doJSON(t, handler, http.MethodPost, "/api/auth/logout", "", cookie)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", cookie)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Login with the wrong password is a uniform 401...
	rec = doJSON(t, handler, http.MethodPost, "/api/auth/login", `{"password":"wrong-password"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// ...and the right password restores access.
	rec = doJSON(t, handler, http.MethodPost, "/api/auth/login", `{"password":"hunter2hunter2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cookie = sessionCookie(t, rec)
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", cookie)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChangePasswordRevokesOldSessions(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"first-pass-1"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	oldCookie := sessionCookie(t, rec)

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/change-password",
		`{"current_password":"first-pass-1","new_password":"second-pass-2"}`, oldCookie)
	require.Equal(t, http.StatusOK, rec.Code)

	// Old session is dead; login requires the new password.
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", oldCookie)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/login", `{"password":"first-pass-1"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/login", `{"password":"second-pass-2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSameOriginGuardBlocksForeignOrigin(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Matching origin passes through to normal handling (401 bad password,
	// not 403 rejected origin).
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+req.Host)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Behind a TLS-terminating proxy the browser's origin is https:// while
// Mizu sees plain HTTP; a proxy may also rewrite Host and pass the public
// host in X-Forwarded-Host. Both must pass, while a foreign host still fails.
func TestSameOriginGuardBehindProxy(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	post := func(host, origin, fwdHost string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"x"}`))
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		if fwdHost != "" {
			req.Header.Set("X-Forwarded-Host", fwdHost)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// TLS terminated at the proxy, Host preserved.
	assert.Equal(t, http.StatusUnauthorized, post("mizu.example.com", "https://mizu.example.com", ""))
	// Proxy rewrote Host to the upstream and sent the public host along.
	assert.Equal(t, http.StatusUnauthorized, post("mizu:8978", "https://mizu.example.com", "mizu.example.com"))
	// A foreign page is still rejected, with or without proxy headers.
	assert.Equal(t, http.StatusForbidden, post("mizu.example.com", "https://evil.example", ""))
	assert.Equal(t, http.StatusForbidden, post("mizu:8978", "https://evil.example", "mizu.example.com"))
}

func TestSessionExpiryRespectedByMiddleware(t *testing.T) {
	handler, db := newAuthTestRouter(t)

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"hunter2hunter2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cookie := sessionCookie(t, rec)

	// Expire every session directly in the DB.
	ctx := t.Context()
	require.NoError(t, db.DeleteAllSessions(ctx))
	require.NoError(t, db.CreateSession(ctx, auth.TokenHash(cookie.Value), time.Now().Add(-time.Minute)))

	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", cookie)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// The password can be changed, never switched off by a session alone.
func TestRemovePasswordEndpointIsGone(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	cookie := signIn(t, handler)
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/remove-password", `{}`, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func putAccess(t *testing.T, handler http.Handler, cookie *http.Cookie, remote, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/auth/access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remote
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func getFrom(handler http.Handler, path, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestExternalSignInMode(t *testing.T) {
	handler, db := newAuthTestRouter(t)
	cookie := signIn(t, handler)
	const proxy = "172.20.0.5:40000"

	// Needs a session and the current password.
	rec := putAccess(t, handler, nil, proxy, `{"mode":"external","trusted_proxies":"172.20.0.5","current_password":"hunter2hunter2"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = putAccess(t, handler, cookie, proxy, `{"mode":"external","trusted_proxies":"172.20.0.5","current_password":"wrong-password"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Refused when the request itself doesn't come through a trusted
	// proxy: switching would lock the caller out.
	rec = putAccess(t, handler, cookie, "192.168.1.30:5000", `{"mode":"external","trusted_proxies":"172.20.0.5","current_password":"hunter2hunter2"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "192.168.1.30")

	rec = putAccess(t, handler, cookie, proxy, `{"mode":"bogus","trusted_proxies":"","current_password":"hunter2hunter2"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = putAccess(t, handler, cookie, proxy, `{"mode":"external","trusted_proxies":"not-an-ip","current_password":"hunter2hunter2"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = putAccess(t, handler, cookie, proxy, `{"mode":"external","trusted_proxies":"172.20.0.0/16","current_password":"hunter2hunter2"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	mode, _ := db.GetSetting(t.Context(), settingAuthMode)
	assert.Equal(t, authModeExternal, mode)

	// Through the proxy: no Mizu session needed.
	assert.Equal(t, http.StatusOK, getFrom(handler, "/api/mappings", proxy).Code)
	assert.Equal(t, authStatus{Configured: true, Authenticated: true, External: true}, getStatus(t, handler, proxy))
	assert.Equal(t, http.StatusOK, getFrom(handler, "/metrics", proxy).Code)

	// Directly (bypassing the proxy, or a DNS-rebinding page running in a
	// LAN browser): refused even with a valid session.
	direct := httptest.NewRequest(http.MethodGet, "/api/mappings", nil)
	direct.RemoteAddr = "192.168.1.30:5000"
	direct.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, direct)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, getStatus(t, handler, "192.168.1.30:5000").Authenticated)

	// Password sign-in is off.
	rec = doJSON(t, handler, http.MethodPost, "/api/auth/login", `{"password":"hunter2hunter2"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// Switching back needs the password again; then a session is required.
	rec = putAccess(t, handler, nil, proxy, `{"mode":"password","trusted_proxies":"","current_password":"hunter2hunter2"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusUnauthorized, getFrom(handler, "/api/mappings", proxy).Code)
}

func TestAccessPolicySurvivesRestart(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "p.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.SetSettings(t.Context(), map[string]string{settingAuthMode: authModeExternal, settingTrustedProxies: "10.0.0.0/8, 172.20.0.5"}))
	p := loadAccessPolicy(t.Context(), db)
	assert.True(t, p.External)
	assert.Equal(t, "10.0.0.0/8, 172.20.0.5", p.TrustedRaw)
}

func TestSettingsRequireKeyWhenURLChanges(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	cookie := signIn(t, handler)

	put := func(body string) *httptest.ResponseRecorder {
		return doJSON(t, handler, http.MethodPut, "/api/settings", body, cookie)
	}
	require.Equal(t, http.StatusOK, put(`{"sonarr_url":"http://sonarr:8989","sonarr_api_key":"saved-key"}`).Code)

	// Same URL (trailing slash, case) without the key: fine.
	assert.Equal(t, http.StatusOK, put(`{"sonarr_url":"http://SONARR:8989/"}`).Code)
	// New URL without the key: refused, so the saved key can't be sent there.
	rec := put(`{"sonarr_url":"http://attacker.example:8989"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "API key")
	// New URL with a new key: fine.
	assert.Equal(t, http.StatusOK, put(`{"sonarr_url":"http://sonarr2:8989","sonarr_api_key":"new-key"}`).Code)

	// URLs that would smuggle a path or reach metadata are refused.
	assert.Equal(t, http.StatusBadRequest, put(`{"sonarr_url":"http://169.254.169.254/latest/?","sonarr_api_key":"k"}`).Code)

	// Same rule for the ntfy token.
	require.Equal(t, http.StatusOK, put(`{"ntfy_server_url":"https://ntfy.example","ntfy_token":"tk_saved"}`).Code)
	assert.Equal(t, http.StatusBadRequest, put(`{"ntfy_server_url":"https://evil.example"}`).Code)
	assert.Equal(t, http.StatusOK, put(`{"ntfy_server_url":"https://evil.example","ntfy_token":""}`).Code, "clearing the token is fine")
}

func TestMutationsRejectFormContentTypes(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	cookie := signIn(t, handler)
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x", "text/plain;charset=UTF-8"} {
		req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"ui_sounds_enabled":false}`))
		req.Header.Set("Content-Type", ct)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code, ct)
	}
	// The artwork upload is the one multipart route.
	req := httptest.NewRequest(http.MethodPost, "/api/mappings/999/artwork/custom/poster", strings.NewReader("--x--"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "reaches the handler (unknown mapping)")
}

func TestSecurityHeaders(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	for _, path := range []string{"/", "/settings", "/api/auth/status"} {
		rec := doJSON(t, handler, http.MethodGet, path, "", nil)
		h := rec.Header()
		assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'none'", path)
		assert.Equal(t, "DENY", h.Get("X-Frame-Options"), path)
		assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"), path)
		assert.Empty(t, h.Get("Strict-Transport-Security"), "no HSTS over plain HTTP")
	}
	// HSTS when a same-host proxy says the browser is on HTTPS.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"))
	// ...but not when an untrusted client claims it.
	req.RemoteAddr = "203.0.113.4:5000"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestSessionCookieSecureFromBrowserOrigin(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"password":"hunter2hunter2"}`))
	req.Header.Set("Content-Type", "application/json")
	// A TLS-terminating proxy that sends no X-Forwarded-Proto.
	req.Header.Set("Origin", "https://"+req.Host)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, sessionCookie(t, rec).Secure)

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/login", `{"password":"hunter2hunter2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, sessionCookie(t, rec).Secure, "plain HTTP keeps a non-Secure cookie so login still works")
}

// /metrics is the conventional scrape path; it must serve the metrics text,
// not the SPA. Unknown API paths must answer a JSON 404, not index.html.
func TestMetricsPathAndAPINotFound(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	cookie := signIn(t, router)

	rec := doJSON(t, router, http.MethodGet, "/metrics", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Basic")

	rec = doJSON(t, router, http.MethodGet, "/metrics", "", cookie)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "mizu_build_info")

	// Prometheus: basic auth with the dashboard password.
	scrape := func(password string) int {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth("prometheus", password)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	assert.Equal(t, http.StatusOK, scrape("hunter2hunter2"))
	assert.Equal(t, http.StatusUnauthorized, scrape("wrong-password"))

	rec = doJSON(t, router, http.MethodGet, "/api/does-not-exist", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}
