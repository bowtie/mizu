package main

import (
	"mizu/internal/models"
	"sort"
	stdsync "sync"
	"time"
)

// episodeCache provides a short-lived in-memory cache for enriched episode
// data so that navigating to an anime page doesn't always hit Sonarr + AFL.
type episodeCache struct {
	mu      stdsync.RWMutex
	entries map[int64]episodeCacheEntry
	ttl     time.Duration
	maxSize int
}

type episodeCacheEntry struct {
	data      []models.EpisodeInfo
	expiresAt time.Time
}

func newEpisodeCache(ttl time.Duration) *episodeCache {
	return &episodeCache{entries: make(map[int64]episodeCacheEntry), ttl: ttl, maxSize: 256}
}

func (c *episodeCache) get(mappingID int64) ([]models.EpisodeInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[mappingID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	cp := make([]models.EpisodeInfo, len(e.data))
	copy(cp, e.data)
	return cp, true
}

func (c *episodeCache) set(mappingID int64, data []models.EpisodeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistic cleanup: remove expired entries to prevent unbounded growth.
	now := time.Now()
	for id, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, id)
		}
	}
	// Enforce max size by evicting the soonest-expiring entries.
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		type idExp struct {
			id        int64
			expiresAt time.Time
		}
		all := make([]idExp, 0, len(c.entries))
		for id, entry := range c.entries {
			all = append(all, idExp{id, entry.expiresAt})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].expiresAt.Before(all[j].expiresAt) })
		evict := len(all) - c.maxSize + 1
		for i := 0; i < evict; i++ {
			delete(c.entries, all[i].id)
		}
	}
	c.entries[mappingID] = episodeCacheEntry{data: data, expiresAt: now.Add(c.ttl)}
}

func (c *episodeCache) invalidate(mappingID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, mappingID)
}

func (c *episodeCache) invalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[int64]episodeCacheEntry)
}

// seriesListCache provides a short-lived in-memory cache for the full
// Sonarr series list. Avoids hitting the Sonarr API on every ShowsPage
// navigation within the same session. Stores SonarrSeriesPublic so cached
// responses are ready to serialize without re-converting.
type seriesListCache struct {
	mu        stdsync.RWMutex
	data      []models.SonarrSeriesPublic
	expiresAt time.Time
	ttl       time.Duration
}

func newSeriesListCache(ttl time.Duration) *seriesListCache {
	return &seriesListCache{ttl: ttl}
}

func (c *seriesListCache) get() ([]models.SonarrSeriesPublic, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if time.Now().After(c.expiresAt) {
		return nil, false
	}
	cp := make([]models.SonarrSeriesPublic, len(c.data))
	copy(cp, c.data)
	return cp, true
}

func (c *seriesListCache) set(data []models.SonarrSeriesPublic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make([]models.SonarrSeriesPublic, len(data))
	copy(c.data, data)
	c.expiresAt = time.Now().Add(c.ttl)
}
