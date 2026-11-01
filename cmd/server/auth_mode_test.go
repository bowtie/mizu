package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"mizu/internal/config"
	"mizu/internal/database"
	syncengine "mizu/internal/sync"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func proxySettings(t *testing.T) config.AuthSettings {
	t.Helper()
	t.Setenv("MIZU_AUTH", "proxy")
	t.Setenv("MIZU_TRUSTED_PROXIES", "10.0.0.0/8")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Auth
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func TestRequireAuthNoneAllowsEverything(t *testing.T) {
	h := requireAuth(config.AuthSettings{Mode: config.AuthNone}, nil)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/settings", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRequireAuthProxy(t *testing.T) {
	h := requireAuth(proxySettings(t), nil)(okHandler())
	do := func(remote, user string) int {
		r := httptest.NewRequest("GET", "/api/settings", nil)
		r.RemoteAddr = remote
		if user != "" {
			r.Header.Set("Remote-User", user)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	if got := do("10.1.2.3:99", "alice"); got != http.StatusNoContent {
		t.Errorf("trusted proxy with user: %d", got)
	}
	if got := do("10.1.2.3:99", ""); got != http.StatusUnauthorized {
		t.Errorf("trusted proxy without user: %d", got)
	}
	if got := do("198.51.100.7:99", "alice"); got != http.StatusUnauthorized {
		t.Errorf("forged header from direct client: %d", got)
	}
}

func TestAuthStatusByMode(t *testing.T) {
	decode := func(h http.HandlerFunc, remote, user string) authStatusResponse {
		r := httptest.NewRequest("GET", "/api/auth/status", nil)
		r.RemoteAddr = remote
		if user != "" {
			r.Header.Set("Remote-User", user)
		}
		rec := httptest.NewRecorder()
		h(rec, r)
		var out authStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	none := decode(handleAuthStatus(config.AuthSettings{Mode: config.AuthNone}, nil), "127.0.0.1:1", "")
	if none.Mode != "none" || !none.Authenticated || none.Configured {
		t.Errorf("none: %+v", none)
	}

	proxy := proxySettings(t)
	ok := decode(handleAuthStatus(proxy, nil), "10.0.0.2:1", "alice")
	if ok.Mode != "proxy" || !ok.Authenticated || ok.User != "alice" {
		t.Errorf("proxy ok: %+v", ok)
	}
	denied := decode(handleAuthStatus(proxy, nil), "198.51.100.7:1", "alice")
	if denied.Authenticated || denied.User != "" {
		t.Errorf("proxy denied: %+v", denied)
	}
}

func TestBuiltinAuthOnly(t *testing.T) {
	for mode, want := range map[config.AuthMode]int{
		config.AuthBuiltin: http.StatusNoContent,
		config.AuthNone:    http.StatusNotFound,
		config.AuthProxy:   http.StatusNotFound,
	} {
		h := builtinAuthOnly(config.AuthSettings{Mode: mode})(okHandler())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/auth/login", nil))
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d", mode, rec.Code, want)
		}
	}
}

func newModeRouter(t *testing.T, settings config.AuthSettings) http.Handler {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	cfg := config.DefaultConfig()
	cfg.SonarrURL = ""
	cfg.DBPath = filepath.Join(t.TempDir(), "cfg.db")
	cfg.Auth = settings
	return setupRouter(cfg, db, syncengine.New(cfg, db), newSchedulerController(cfg.ScanInterval), t.Context(), nil)
}

func TestRouterPasswordEndpointsDisabledWhenManaged(t *testing.T) {
	for _, settings := range []config.AuthSettings{
		{Mode: config.AuthNone},
		proxySettings(t),
	} {
		h := newModeRouter(t, settings)
		for _, path := range []string{"/api/auth/setup", "/api/auth/login", "/api/auth/logout", "/api/auth/change-password"} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"password":"abcdefgh"}`))
			req.RemoteAddr = "10.0.0.2:1"
			req.Header.Set("Remote-User", "alice")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code, "%s in %s mode", path, settings.Mode)
		}
	}
}

func TestRouterProxyModeGatesAPI(t *testing.T) {
	h := newModeRouter(t, proxySettings(t))
	get := func(path, remote, user string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remote
		if user != "" {
			req.Header.Set("Remote-User", user)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	assert.Equal(t, http.StatusOK, get("/api/settings", "10.0.0.2:1", "alice"))
	assert.Equal(t, http.StatusUnauthorized, get("/api/settings", "10.0.0.2:1", ""))
	assert.Equal(t, http.StatusUnauthorized, get("/api/settings", "198.51.100.7:1", "alice"))
	assert.Equal(t, http.StatusOK, get("/api/health", "198.51.100.7:1", ""), "health stays public")
}

func TestRouterNeverReturnsStoredKeys(t *testing.T) {
	h := newModeRouter(t, config.AuthSettings{Mode: config.AuthNone})
	for _, path := range []string{"/api/settings", "/api/bootstrap"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)
		body := rec.Body.String()
		for _, key := range []string{`"sonarr_api_key":`, `"tmdb_api_key":`, `"fanart_api_key":`} {
			assert.NotContains(t, body, key, path)
		}
	}
}
