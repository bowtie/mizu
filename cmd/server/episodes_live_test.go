package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"
)

// A new show's episodes are listed live until its first sync. The list must
// not change when that sync replaces it: same numbers, titles and types.
func TestNewShowListsEpisodesAsItsFirstSyncWill(t *testing.T) {
	abs := func(n int) *int { return &n }
	aired := time.Now().AddDate(-1, 0, 0)
	sonarrEps := []models.SonarrEpisode{
		{ID: 1, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: 1, AbsoluteEpisodeNumber: abs(11), Title: "The Beginning", AirDateUTC: aired},
		{ID: 2, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: 2, AbsoluteEpisodeNumber: abs(12), Title: "Beach Day", AirDateUTC: aired},
		// Not numbered by Sonarr yet: listed as 13, after 12.
		{ID: 3, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: 3, Title: "Brand New", AirDateUTC: aired},
		// AFL lists the special as episode 50.
		{ID: 4, SeriesID: 100, SeasonNumber: 0, EpisodeNumber: 1, Title: "Movie Special", AirDateUTC: aired},
	}
	sonarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/series/100":
			_ = json.NewEncoder(w).Encode(models.SonarrSeries{ID: 100, Title: "Test Show"})
		case "/api/v3/episode":
			_ = json.NewEncoder(w).Encode(sonarrEps)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(sonarr.Close)
	afl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		row := `<tr><td class="Number">%d</td><td class="Title"><a href="#">%s</a></td><td class="Type"><span>%s</span></td></tr>`
		rows := fmt.Sprintf(row, 11, "The Beginning!", "canon") + fmt.Sprintf(row, 12, "Beach Day", "filler") + fmt.Sprintf(row, 50, "Movie Special", "filler")
		_, _ = io.WriteString(w, `<html><body><span property="dc:title" content="Test Show"></span><table class="EpisodeList"><tbody>`+rows+`</tbody></table></body></html>`)
	}))
	t.Cleanup(afl.Close)

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarr.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 5 * time.Second
	engine := syncengine.New(cfg, db)
	engine.ScraperClient().SetBaseURL(afl.URL)
	mapping := &models.ShowMapping{
		SonarrSeriesID: 100, SonarrTitle: "Test Show", AFLSlug: "test-show", Enabled: true,
		FillerAction: models.ActionMonitor, MixedAction: models.ActionMonitor, AnimeCanonAction: models.ActionMonitor,
		StartMode: models.StartModeFromBeginning, WatermarkMode: models.WatermarkModeManual,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), mapping))

	type listed struct {
		Num         int
		Title, Type string
	}
	byEpisode := func(eps []models.EpisodeInfo) map[int]listed {
		m := map[int]listed{}
		for _, e := range eps {
			m[e.SonarrEpisodeID] = listed{e.EpisodeNum, e.Title, string(e.Type)}
		}
		return m
	}

	live, herr := loadEpisodes(context.Background(), db, engine, mapping.ID, nil)
	require.Nil(t, herr)

	done, ok := engine.TryAcquireRun()
	require.True(t, ok)
	require.NoError(t, engine.RunResyncClaimed(context.Background(), mapping.ID))
	done()
	synced, err := db.GetEpisodes(context.Background(), mapping.ID)
	require.NoError(t, err)

	assert.Equal(t, map[int]listed{
		1: {11, "The Beginning!", "canon"},
		2: {12, "Beach Day", "filler"},
		3: {13, "Brand New", "unknown"},
		4: {50, "Movie Special", "filler"},
	}, byEpisode(synced))
	assert.Equal(t, byEpisode(synced), byEpisode(live))
}
