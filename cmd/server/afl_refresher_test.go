package main

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/models"
	"mizu/internal/scraper"
)

func TestAFLCacheRefreshDue(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	assert.True(t, aflCacheRefreshDue(0, now, now), "empty cache")
	assert.True(t, aflCacheRefreshDue(371, time.Time{}, now), "never refreshed")
	assert.True(t, aflCacheRefreshDue(371, now.Add(-scraper.ShowListRefreshInterval), now), "a full interval old")
	assert.False(t, aflCacheRefreshDue(371, now.Add(-time.Hour), now), "fresh")
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

	// A stale cache with one show is refreshed, picking up the new one.
	_, err := db.ReplaceCachedAFLShows(t.Context(), []models.AFLShow{{Slug: "one-piece", Title: "One Piece"}},
		time.Now().Add(-scraper.ShowListRefreshInterval-time.Minute))
	require.NoError(t, err)
	require.True(t, refreshAFLCacheIfDue(t.Context(), engine, db, &bgWG))
	count, refreshedAt, err := db.GetAFLShowCacheStatus(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.WithinDuration(t, time.Now(), refreshedAt, time.Minute)

	// Now it's fresh, so the next check leaves AnimeFillerList alone.
	assert.False(t, refreshAFLCacheIfDue(t.Context(), engine, db, &bgWG))
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
	assert.False(t, refreshAFLCacheIfDue(t.Context(), engine, db, &bgWG))
	assert.Zero(t, fetches.Load())
}
