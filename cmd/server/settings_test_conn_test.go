package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
)

// keyRecorder is a stub service that records the credential headers it saw.
func keyRecorder(t *testing.T, header string, seen *atomic.Value) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get(header))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"4.0.0"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// An empty api_key reuses the saved key only for the saved URL; any other
// URL must be refused before a request carrying the key leaves the server.
func TestTestSonarrKeepsSavedKeyOnSavedURL(t *testing.T) {
	var seenSaved, seenOther atomic.Value
	saved := keyRecorder(t, "X-Api-Key", &seenSaved)
	other := keyRecorder(t, "X-Api-Key", &seenOther)

	cfg := config.DefaultConfig()
	cfg.SonarrURL = saved.URL
	cfg.SonarrAPIKey = "secret-key"

	rec := httptest.NewRecorder()
	handleTestSonarr(cfg)(rec, httptest.NewRequest(http.MethodPost, "/api/sonarr/test",
		strings.NewReader(`{"url":"`+other.URL+`","api_key":""}`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Nil(t, seenOther.Load(), "saved key must not be sent to a different URL")

	rec = httptest.NewRecorder()
	handleTestSonarr(cfg)(rec, httptest.NewRequest(http.MethodPost, "/api/sonarr/test",
		strings.NewReader(`{"url":"`+saved.URL+`/","api_key":""}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "secret-key", seenSaved.Load())
}

// The saved ntfy token is dropped when the test targets another server.
func TestTestNtfyDropsSavedTokenForOtherServer(t *testing.T) {
	var seen atomic.Value
	other := keyRecorder(t, "Authorization", &seen)

	cfg := config.DefaultConfig()
	cfg.NtfyServerURL = "https://ntfy.example.com"
	cfg.NtfyTopic = "mizu"
	cfg.NtfyToken = "secret-token"

	rec := httptest.NewRecorder()
	handleTestNtfy(cfg)(rec, httptest.NewRequest(http.MethodPost, "/api/ntfy/test",
		strings.NewReader(`{"server_url":"`+other.URL+`","topic":"mizu"}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "", seen.Load(), "saved token must not be sent to a different server")
}
