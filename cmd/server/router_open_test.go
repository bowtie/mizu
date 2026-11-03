package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lanHost is how a LAN browser addresses Mizu: by IP, which a
// DNS-rebinding page can't use.
const lanHost = "192.168.1.10:8978"

func doHost(handler http.Handler, method, path, host, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = host
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

func TestFirstRunCanTurnSignInOff(t *testing.T) {
	handler, db := newAuthTestRouter(t)

	rec := doHost(handler, http.MethodPost, "/api/auth/setup", lanHost, `{"open":true}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	mode, _ := db.GetSetting(t.Context(), settingAuthMode)
	assert.Equal(t, string(authModeOpen), mode)

	assert.Equal(t, http.StatusOK, doHost(handler, http.MethodGet, "/api/mappings", lanHost, "", nil).Code)
	assert.Equal(t, http.StatusOK, doHost(handler, http.MethodGet, "/metrics", lanHost, "", nil).Code)

	var st authStatusResponse
	require.NoError(t, json.Unmarshal(doHost(handler, http.MethodGet, "/api/auth/status", lanHost, "", nil).Body.Bytes(), &st))
	assert.Equal(t, authStatusResponse{Configured: true, Authenticated: true, Mode: authModeOpen}, st)

	// Setup can't run twice, either way.
	rec = doHost(handler, http.MethodPost, "/api/auth/setup", lanHost, `{"password":"hunter2hunter2"}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestFirstRunOpenAfterPasswordRefused(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	signIn(t, handler)
	rec := doHost(handler, http.MethodPost, "/api/auth/setup", lanHost, `{"open":true}`, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, http.StatusUnauthorized, doHost(handler, http.MethodGet, "/api/mappings", lanHost, "", nil).Code)
}

// With sign-in off, only IP addresses, localhost and listed hostnames are
// answered, so a DNS-rebinding page under its own hostname gets nothing.
func TestOpenModeRefusesUnknownHosts(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	rec := doHost(handler, http.MethodPost, "/api/auth/setup", "mizu.lan:8978", `{"open":true}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	for host, want := range map[string]int{
		"mizu.lan:8978":       http.StatusOK, // setup was opened under it
		"MIZU.LAN":            http.StatusOK,
		"127.0.0.1:8978":      http.StatusOK,
		"[::1]:8978":          http.StatusOK,
		"localhost:8978":      http.StatusOK,
		"rebind.attacker.com": http.StatusMisdirectedRequest,
		"mizu.lan.evil.com":   http.StatusMisdirectedRequest,
	} {
		assert.Equal(t, want, doHost(handler, http.MethodGet, "/api/mappings", host, "", nil).Code, host)
	}
}

func TestSwitchingSignInOffAndBackOn(t *testing.T) {
	handler, db := newAuthTestRouter(t)
	cookie := signIn(t, handler)

	// Turning it off needs the current password.
	rec := doHost(handler, http.MethodPut, "/api/auth/access", lanHost, `{"mode":"open","current_password":"wrong-password"}`, cookie)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	// The hostname this browser uses is kept answering without listing it.
	rec = doHost(handler, http.MethodPut, "/api/auth/access", "mizu.lan", `{"mode":"open","allowed_hosts":"nas.local","current_password":"hunter2hunter2"}`, cookie)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"password_set":false`)
	assert.Contains(t, rec.Body.String(), `"allowed_hosts":"nas.local, mizu.lan"`)
	assert.Equal(t, authStatusResponse{Configured: true, Authenticated: true, Mode: authModeOpen}, accessUpdateStatus(t, rec))
	hash, _ := db.GetSetting(t.Context(), "auth_password_hash")
	assert.Empty(t, hash, "turning sign-in off removes the password")
	assert.Equal(t, http.StatusOK, doHost(handler, http.MethodGet, "/api/mappings", "mizu.lan", "", nil).Code)

	// Exporting secrets has no password to ask for.
	rec = doHost(handler, http.MethodPost, "/api/backup/export", lanHost, `{"include_secrets":true}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"includes_secrets":true`)

	// Back to a password: needs a new one, and signs this browser in.
	rec = doHost(handler, http.MethodPut, "/api/auth/access", lanHost, `{"mode":"password","new_password":"short"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doHost(handler, http.MethodPut, "/api/auth/access", lanHost, `{"mode":"password","new_password":"correct-horse"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	newCookie := sessionCookie(t, rec)
	assert.Equal(t, authStatusResponse{Configured: true, Authenticated: true, Mode: authModePassword}, accessUpdateStatus(t, rec))

	assert.Equal(t, http.StatusUnauthorized, doHost(handler, http.MethodGet, "/api/mappings", lanHost, "", nil).Code)
	assert.Equal(t, http.StatusOK, doHost(handler, http.MethodGet, "/api/mappings", lanHost, "", newCookie).Code)
	// The old session died with the old password.
	assert.Equal(t, http.StatusUnauthorized, doHost(handler, http.MethodGet, "/api/mappings", lanHost, "", cookie).Code)
}

func TestOpenModeSurvivesRestart(t *testing.T) {
	_, db := newAuthTestRouter(t)
	require.NoError(t, db.SetSettings(t.Context(), map[string]string{settingAuthMode: string(authModeOpen), settingAllowedHosts: "Mizu.lan:80, nas.local"}))
	p := loadAccessPolicy(t.Context(), db)
	assert.Equal(t, authModeOpen, p.Mode)
	assert.Equal(t, []string{"mizu.lan", "nas.local"}, p.AllowedHosts)
}
