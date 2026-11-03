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

func webhookToken(t *testing.T, handler http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := doJSON(t, handler, http.MethodGet, "/api/settings/webhook", "", cookie)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Token)
	return body.Token
}

func postWebhook(handler http.Handler, host, body string, auth func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, sonarrWebhookPath, strings.NewReader(body))
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	if auth != nil {
		auth(req)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func basicAuth(password string) func(*http.Request) {
	return func(r *http.Request) { r.SetBasicAuth("sonarr", password) }
}

func TestWebhookNeedsItsToken(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	cookie := signIn(t, handler)

	// No token exists until Settings asks for one.
	assert.Equal(t, http.StatusUnauthorized, postWebhook(handler, lanHost, `{"eventType":"Test"}`, basicAuth("")).Code)

	token := webhookToken(t, handler, cookie)
	assert.Equal(t, token, webhookToken(t, handler, cookie), "the token is created once")

	test := `{"eventType":"Test"}`
	assert.Equal(t, http.StatusUnauthorized, postWebhook(handler, lanHost, test, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, postWebhook(handler, lanHost, test, basicAuth("wrong")).Code)
	assert.Equal(t, http.StatusOK, postWebhook(handler, lanHost, test, basicAuth(token)).Code)
	assert.Equal(t, http.StatusOK, postWebhook(handler, lanHost, test, func(r *http.Request) {
		r.Header.Set("X-Api-Key", token)
	}).Code)

	// Events for shows Mizu doesn't track are acknowledged and dropped.
	rec := postWebhook(handler, lanHost, `{"eventType":"Grab","series":{"id":42},"episodes":[{"id":1}]}`, basicAuth(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ignored")

	// The token page itself needs sign-in.
	assert.Equal(t, http.StatusUnauthorized, doJSON(t, handler, http.MethodGet, "/api/settings/webhook", "", nil).Code)
}

func TestWebhookTokenRegenerates(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	cookie := signIn(t, handler)
	old := webhookToken(t, handler, cookie)

	rec := doJSON(t, handler, http.MethodPost, "/api/settings/webhook/regenerate", "", cookie)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	fresh := webhookToken(t, handler, cookie)
	assert.NotEqual(t, old, fresh)

	test := `{"eventType":"Test"}`
	assert.Equal(t, http.StatusUnauthorized, postWebhook(handler, lanHost, test, basicAuth(old)).Code)
	assert.Equal(t, http.StatusOK, postWebhook(handler, lanHost, test, basicAuth(fresh)).Code)
}

// Sonarr in Docker reaches Mizu by its container name, which open mode
// wouldn't otherwise answer to. The token stops a rebinding page instead.
func TestWebhookAnswersAnyHostWithSignInOff(t *testing.T) {
	handler, _ := newAuthTestRouter(t)
	rec := doHost(handler, http.MethodPost, "/api/auth/setup", lanHost, `{"open":true}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = doHost(handler, http.MethodGet, "/api/settings/webhook", lanHost, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	token := body.Token

	assert.Equal(t, http.StatusMisdirectedRequest, doHost(handler, http.MethodGet, "/api/mappings", "mizu:8978", "", nil).Code)
	assert.Equal(t, http.StatusOK, postWebhook(handler, "mizu:8978", `{"eventType":"Test"}`, basicAuth(token)).Code)
	assert.Equal(t, http.StatusUnauthorized, postWebhook(handler, "mizu:8978", `{"eventType":"Test"}`, nil).Code)
}
