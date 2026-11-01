package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterNeverReturnsStoredKeys(t *testing.T) {
	h, _ := newAuthTestRouter(t)
	for _, path := range []string{"/api/settings", "/api/mappings"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)
		body := rec.Body.String()
		for _, key := range []string{`"sonarr_api_key":`, `"tmdb_api_key":`, `"fanart_api_key":`} {
			assert.NotContains(t, body, key, path)
		}
	}
}
