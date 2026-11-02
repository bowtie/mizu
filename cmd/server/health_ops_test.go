package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/auth"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

func TestHealthOKReturns200AndCachesSonarrProbe(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	var hits int
	var mu sync.Mutex
	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"3.0.6"}`))
	}))
	t.Cleanup(sonarrSrv.Close)

	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarrSrv.URL
	engine := newSyncRaceEngine(t, sonarrSrv.URL, db)
	scheduler := newSchedulerController(cfg.ScanInterval)

	statusCache := newSonarrStatusCache(healthSonarrTTL)
	handler := handleHealth(cfg, db, scheduler, engine, statusCache, auth.NewManager(db), newAccessControl(accessPolicy{}))

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])

	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, hits, "second health call within TTL must reuse the cached probe")
}

func TestHealthDegradedReturns503(t *testing.T) {
	// Port 1 is unreachable — the probe fails fast.
	cfg := config.DefaultConfig()
	cfg.SonarrURL = "http://127.0.0.1:1"

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	engine := newSyncRaceEngine(t, cfg.SonarrURL, db)

	rec := httptest.NewRecorder()
	handleHealth(cfg, db, newSchedulerController(cfg.ScanInterval), engine, newSonarrStatusCache(healthSonarrTTL), auth.NewManager(db), newAccessControl(accessPolicy{}))(
		rec, httptest.NewRequest(http.MethodGet, "/api/health", nil),
	)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body["status"])
}

func TestHealthNoSonarrConfiguredIsOK(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	cfg := config.DefaultConfig()
	cfg.SonarrURL = ""
	engine := newSyncRaceEngine(t, "", db)

	rec := httptest.NewRecorder()
	handleHealth(cfg, db, newSchedulerController(cfg.ScanInterval), engine, newSonarrStatusCache(healthSonarrTTL), auth.NewManager(db), newAccessControl(accessPolicy{}))(
		rec, httptest.NewRequest(http.MethodGet, "/api/health", nil),
	)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func countAllSyncLogs(t *testing.T, db *database.DB, ctx context.Context) int {
	t.Helper()
	page, err := db.GetSyncLogsPage(ctx, 100, 0)
	require.NoError(t, err)
	total := len(page.Items)
	offset := page.NextOffset
	for page.HasMore {
		page, err = db.GetSyncLogsPage(ctx, 100, offset)
		require.NoError(t, err)
		total += len(page.Items)
		offset = page.NextOffset
	}
	return total
}

func TestPruneSyncLogsKeepsNewestWindow(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()

	// Attach items to the very first (oldest) log so cascade deletion is
	// observable too.
	oldestID, err := db.CreateSyncLog(ctx, "manual")
	require.NoError(t, err)
	require.NoError(t, db.ReplaceSyncLogItems(ctx, oldestID, []models.SyncLogItem{
		{SonarrTitle: "Old Show", Status: "checked"},
	}))

	for i := 0; i < database.SyncLogRetentionCount+10; i++ {
		_, err := db.CreateSyncLog(ctx, "manual")
		require.NoError(t, err)
	}

	assert.Equal(t, database.SyncLogRetentionCount, countAllSyncLogs(t, db, ctx),
		"retention must cap the number of stored logs")

	itemsByID, err := db.GetSyncLogItems(ctx, []int64{oldestID})
	require.NoError(t, err)
	assert.Empty(t, itemsByID[oldestID], "pruned log's items must be cascade-deleted")
}

func TestSweepStaleRunningSyncLogs(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()

	stuckID, err := db.CreateSyncLog(ctx, "scheduled")
	require.NoError(t, err)

	doneID, err := db.CreateSyncLog(ctx, "scheduled")
	require.NoError(t, err)
	require.NoError(t, db.CompleteSyncLog(ctx, doneID, "success", 3, 5, ""))

	n, err := db.SweepStaleRunningSyncLogs(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	logs, err := db.GetSyncLogsPage(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs.Items, 2)

	byID := make(map[int64]models.SyncLog, len(logs.Items))
	for _, l := range logs.Items {
		byID[l.ID] = l
	}
	stuck := byID[stuckID]
	assert.Equal(t, "failed", stuck.Status)
	assert.NotNil(t, stuck.CompletedAt)
	assert.Contains(t, stuck.Errors, "interrupted by restart")

	done := byID[doneID]
	assert.Equal(t, "success", done.Status, "completed logs must not be touched")
	assert.Empty(t, done.Errors)
}

// Liveness ignores Sonarr: an unreachable Sonarr makes /api/health 503,
// but /api/health/live stays 200 so a container supervisor doesn't
// restart Mizu for Sonarr's outage.
func TestLivenessIgnoresSonarr(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health/live", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestLivenessReportsDatabaseDown(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	rec := httptest.NewRecorder()
	handleLiveness(db)(rec, httptest.NewRequest(http.MethodGet, "/api/health/live", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestHealthHidesDetailsFromStrangers(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	cfg := config.DefaultConfig()
	cfg.SonarrURL = ""
	engine := newSyncRaceEngine(t, "", db)
	mgr := auth.NewManager(db)
	h := handleHealth(cfg, db, newSchedulerController(cfg.ScanInterval), engine, newSonarrStatusCache(healthSonarrTTL), mgr, newAccessControl(accessPolicy{}))

	token, _, err := mgr.Setup(t.Context(), "health-check-pass")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec = httptest.NewRecorder()
	h(rec, req)
	assert.Contains(t, rec.Body.String(), `"version"`)
}
