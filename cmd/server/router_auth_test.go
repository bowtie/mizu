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

func TestAuthDisabledWhenUnconfigured(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	rec := doJSON(t, handler, http.MethodGet, "/api/mappings", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = doJSON(t, handler, http.MethodGet, "/api/auth/status", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var status struct {
		Configured    bool `json:"configured"`
		Authenticated bool `json:"authenticated"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	assert.False(t, status.Configured)
	assert.False(t, status.Authenticated)
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

	// Health/version stay public for monitoring.
	rec = doJSON(t, handler, http.MethodGet, "/api/health", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = doJSON(t, handler, http.MethodGet, "/api/version", "", nil)
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

func TestRemovePasswordOpensDashboard(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"first-pass-1"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cookie := sessionCookie(t, rec)

	// Needs a signed-in session, not the password.
	rec = doJSON(t, handler, http.MethodPost, "/api/auth/remove-password", `{}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/remove-password", `{}`, cookie)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = doJSON(t, handler, http.MethodGet, "/api/mappings", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code, "open without a session")
}

// /metrics is the conventional scrape path; it must serve the metrics text,
// not the SPA. Unknown API paths must answer a JSON 404, not index.html.
func TestMetricsPathAndAPINotFound(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	rec := doJSON(t, router, http.MethodGet, "/metrics", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "mizu_build_info")

	rec = doJSON(t, router, http.MethodGet, "/api/does-not-exist", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}
