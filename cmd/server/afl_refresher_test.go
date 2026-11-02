package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/models"
)

func TestAFLCacheRefreshDue(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	week := config.AFLRefreshDuration("7d")
	assert.True(t, aflCacheRefreshDue(0, now, now, week), "empty cache")
	assert.True(t, aflCacheRefreshDue(371, time.Time{}, now, week), "never refreshed")
	assert.True(t, aflCacheRefreshDue(371, now.Add(-week), now, week), "a full interval old")
	assert.False(t, aflCacheRefreshDue(371, now.Add(-24*time.Hour), now, week), "fresh")
	assert.False(t, aflCacheRefreshDue(371, now.Add(-365*24*time.Hour), now, 0), "refreshes off")
	assert.True(t, aflCacheRefreshDue(0, now, now, 0), "refreshes off still fill an empty cache")
}

func TestRefreshAFLCacheIfDue(t *testing.T) {
	resetAFLRefreshGlobals(t)
	var fetches atomic.Int32
	engine, db := newAFLRefreshTestEngine(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(aflShowListHTML))
	}))
	var bgWG sync.WaitGroup

	cfg := config.DefaultConfig()

	// A stale cache with one show is refreshed, picking up the new one.
	_, err := db.ReplaceCachedAFLShows(t.Context(), []models.AFLShow{{Slug: "one-piece", Title: "One Piece"}},
		time.Now().Add(-config.AFLRefreshDuration(cfg.AFLRefreshInterval)-time.Minute))
	require.NoError(t, err)
	require.True(t, refreshAFLCacheIfDue(t.Context(), cfg, engine, db, &bgWG))
	count, refreshedAt, err := db.GetAFLShowCacheStatus(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.WithinDuration(t, time.Now(), refreshedAt, time.Minute)

	// Now it's fresh, so the next check leaves AnimeFillerList alone.
	assert.False(t, refreshAFLCacheIfDue(t.Context(), cfg, engine, db, &bgWG))
	assert.Equal(t, int32(1), fetches.Load())
}

func TestRefreshAFLCacheIfDueSkipsWhileRefreshing(t *testing.T) {
	resetAFLRefreshGlobals(t)
	var fetches atomic.Int32
	engine, db := newAFLRefreshTestEngine(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
	}))
	var bgWG sync.WaitGroup

	aflCacheRefreshing.Store(true) // a manual refresh is in flight
	assert.False(t, refreshAFLCacheIfDue(t.Context(), config.DefaultConfig(), engine, db, &bgWG))
	assert.Zero(t, fetches.Load())
}

func TestAFLRefreshIntervalSetting(t *testing.T) {
	handler, db := newAuthTestRouter(t)
	cookie := signIn(t, handler)

	rec := doJSON(t, handler, http.MethodPut, "/api/settings", `{"afl_refresh_interval":"hourly"}`, cookie)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(t, handler, http.MethodPut, "/api/settings", `{"afl_refresh_interval":"30d"}`, cookie)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored, _ := db.GetSetting(t.Context(), "afl_refresh_interval")
	assert.Equal(t, "30d", stored)

	var got map[string]any
	require.NoError(t, json.Unmarshal(doJSON(t, handler, http.MethodGet, "/api/settings", "", cookie).Body.Bytes(), &got))
	assert.Equal(t, "30d", got["afl_refresh_interval"])
}
