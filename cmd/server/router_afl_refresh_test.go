package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	syncengine "mizu/internal/sync"
)

// aflShowListHTML is the Anime Filler List show-index page shape the
// scraper parses: #ShowList containing grouped <li><a> entries.
const aflShowListHTML = `<!DOCTYPE html><html><body>
<div id="ShowList">
  <div class="Group"><ul>
    <li><a href="/shows/one-piece">One Piece</a></li>
    <li><a href="/shows/bleach">Bleach</a></li>
  </ul></div>
</div>
</body></html>`

func newAFLRefreshTestEngine(t *testing.T, aflHandler http.Handler) (*syncengine.Engine, *database.DB) {
	t.Helper()
	aflSrv := httptest.NewServer(aflHandler)
	t.Cleanup(aflSrv.Close)

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	e := newSyncRaceEngine(t, "http://sonarr.invalid", db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)
	return e, db
}

// resetAFLRefreshGlobals normalizes package-level refresh state between
// tests so CAS assertions are deterministic.
func resetAFLRefreshGlobals(t *testing.T) {
	t.Helper()
	aflCacheRefreshing.Store(false)
	aflCacheLastError.Store(nil)
	t.Cleanup(func() {
		aflCacheRefreshing.Store(false)
		aflCacheLastError.Store(nil)
	})
}

func TestRefreshAFLCacheReturns202AndPopulatesCacheInBackground(t *testing.T) {
	resetAFLRefreshGlobals(t)
	engine, db := newAFLRefreshTestEngine(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(aflShowListHTML))
	}))

	var bgWG sync.WaitGroup
	rec := httptest.NewRecorder()
	handleRefreshAFLCache(engine, db, t.Context(), &bgWG)(
		rec, httptest.NewRequest(http.MethodPost, "/api/settings/afl-cache/refresh", nil),
	)

	// The response must come back before the scrape finishes.
	require.Equal(t, http.StatusAccepted, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "refreshing", body["status"])

	bgWG.Wait()

	// Cache populated...
	count, refreshedAt, err := db.GetAFLShowCacheStatus(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.False(t, refreshedAt.IsZero())

	// ...activity log recorded as success...
	logs, err := db.GetSyncLogsPage(t.Context(), 10, 0)
	require.NoError(t, err)
	require.Len(t, logs.Items, 1)
	assert.Equal(t, "success", logs.Items[0].Status)

	// ...and the status endpoint reports idle with no error.
	statusRec := httptest.NewRecorder()
	handleAFLCacheRefreshStatus(statusRec, httptest.NewRequest(http.MethodGet, "/api/settings/afl-cache/status", nil))
	require.Equal(t, http.StatusOK, statusRec.Code)
	var status map[string]interface{}
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &status))
	assert.Equal(t, false, status["refreshing"])
	assert.NotContains(t, status, "last_error")
}

func TestRefreshAFLCacheFailureSurfacesViaStatusAndActivityLog(t *testing.T) {
	resetAFLRefreshGlobals(t)
	engine, db := newAFLRefreshTestEngine(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "scrape exploded", http.StatusInternalServerError)
	}))

	var bgWG sync.WaitGroup
	rec := httptest.NewRecorder()
	handleRefreshAFLCache(engine, db, t.Context(), &bgWG)(
		rec, httptest.NewRequest(http.MethodPost, "/api/settings/afl-cache/refresh", nil),
	)
	require.Equal(t, http.StatusAccepted, rec.Code, "failure must not block the HTTP response")

	bgWG.Wait()

	statusRec := httptest.NewRecorder()
	handleAFLCacheRefreshStatus(statusRec, httptest.NewRequest(http.MethodGet, "/api/settings/afl-cache/status", nil))
	require.Equal(t, http.StatusOK, statusRec.Code)
	var status map[string]interface{}
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &status))
	assert.Equal(t, false, status["refreshing"])
	require.Contains(t, status, "last_error")
	assert.Contains(t, status["last_error"], "unexpected status 500")

	logs, err := db.GetSyncLogsPage(t.Context(), 10, 0)
	require.NoError(t, err)
	require.Len(t, logs.Items, 1)
	assert.Equal(t, "failed", logs.Items[0].Status)
}

func TestRefreshAFLCacheRejectsConcurrentTrigger(t *testing.T) {
	resetAFLRefreshGlobals(t)

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	engine, db := newAFLRefreshTestEngine(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(aflShowListHTML))
	}))

	var bgWG sync.WaitGroup

	first := httptest.NewRecorder()
	handleRefreshAFLCache(engine, db, t.Context(), &bgWG)(
		first, httptest.NewRequest(http.MethodPost, "/api/settings/afl-cache/refresh", nil),
	)
	require.Equal(t, http.StatusAccepted, first.Code)
	<-started // scrape is parked mid-flight

	second := httptest.NewRecorder()
	handleRefreshAFLCache(engine, db, t.Context(), &bgWG)(
		second, httptest.NewRequest(http.MethodPost, "/api/settings/afl-cache/refresh", nil),
	)
	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Contains(t, second.Body.String(), "already in progress")

	close(release)
	bgWG.Wait()
}

func TestAFLShowListParsingEndToEnd(t *testing.T) {
	resetAFLRefreshGlobals(t)
	engine, db := newAFLRefreshTestEngine(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/shows") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(aflShowListHTML))
	}))

	var bgWG sync.WaitGroup
	rec := httptest.NewRecorder()
	handleRefreshAFLCache(engine, db, t.Context(), &bgWG)(
		rec, httptest.NewRequest(http.MethodPost, "/api/settings/afl-cache/refresh", nil),
	)
	require.Equal(t, http.StatusAccepted, rec.Code)
	bgWG.Wait()

	shows, _, err := db.GetCachedAFLShows(t.Context())
	require.NoError(t, err)
	require.Len(t, shows, 2)
	slugs := []string{shows[0].Slug, shows[1].Slug}
	assert.ElementsMatch(t, []string{"one-piece", "bleach"}, slugs)
}
