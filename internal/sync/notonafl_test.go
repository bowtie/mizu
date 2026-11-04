package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
)

// bannedFixture is a show where TVDB keeps two episodes AFL doesn't count
// (Banned A and B, Sonarr 4 and 5) ahead of AFL's episodes 4-6, which
// Sonarr numbers 6-8.
type bannedFixture struct {
	mu        stdsync.Mutex
	monitored map[int]bool
	aired     time.Time
}

var bannedTitles = []string{"Opening Day", "Second Step", "Third Time", "Banned A", "Banned B", "Fourth Wall", "Fifth Gear", "Sixth Sense"}

func (f *bannedFixture) sonarr(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/episode":
		eps := make([]models.SonarrEpisode, 0, len(bannedTitles))
		for i, title := range bannedTitles {
			n := i + 1
			eps = append(eps, models.SonarrEpisode{ID: 9000 + n, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: &n, Title: title, Monitored: f.monitored[9000+n], AirDateUTC: f.aired})
		}
		_ = json.NewEncoder(w).Encode(eps)
	case r.Method == http.MethodPut && r.URL.Path == "/api/v3/episode/monitor":
		var body struct {
			EpisodeIDs []int `json:"episodeIds"`
			Monitored  bool  `json:"monitored"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, id := range body.EpisodeIDs {
			f.monitored[id] = body.Monitored
		}
	default:
		http.NotFound(w, r)
	}
}

func (f *bannedFixture) afl(w http.ResponseWriter, _ *http.Request) {
	var rows strings.Builder
	n := 0
	for _, title := range bannedTitles {
		if strings.HasPrefix(title, "Banned") {
			continue
		}
		n++
		fmt.Fprintf(&rows, `<tr><td class="Number">%d</td><td class="Title"><a href="#">%s</a></td><td class="Type"><span>canon</span></td></tr>`, n, title)
	}
	_, _ = io.WriteString(w, `<html><body><span property="dc:title" content="Test Show"></span><table class="EpisodeList"><tbody>`+rows.String()+`</tbody></table></body></html>`)
}

func newBannedEngine(t *testing.T, hold string, aired time.Time, watermark int) (*Engine, *bannedFixture, int64) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	f := &bannedFixture{monitored: map[int]bool{}, aired: aired}
	for i := range bannedTitles {
		f.monitored[9001+i] = true
	}
	sonarrSrv := httptest.NewServer(http.HandlerFunc(f.sonarr))
	t.Cleanup(sonarrSrv.Close)
	aflSrv := httptest.NewServer(http.HandlerFunc(f.afl))
	t.Cleanup(aflSrv.Close)

	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarrSrv.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 5 * time.Second
	cfg.HoldUnclassified = hold
	e := New(cfg, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)

	m := &models.ShowMapping{
		SonarrSeriesID: 100, SonarrTitle: "Test Show", AFLSlug: "test-show", AFLTitle: "Test Show", Enabled: true,
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor, AnimeCanonAction: models.ActionMonitor,
		StartMode: models.StartModeFromBeginning, WatermarkMode: models.WatermarkModeManual, SyncWatermark: watermark,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	return e, f, m.ID
}

func TestEpisodesAFLDoesNotCountSitBehindTheWatermarkByPosition(t *testing.T) {
	// Watched up to AFL 4 ("Fourth Wall"); both banned episodes aired
	// before it, though Sonarr numbers Banned B 5.
	e, f, id := newBannedEngine(t, "off", time.Now().AddDate(-1, 0, 0), 4)
	resync(t, e, id)

	assert.False(t, f.monitored[9004], "Banned A is behind the watermark")
	assert.False(t, f.monitored[9005], "Banned B is behind the watermark")
	assert.False(t, f.monitored[9006], "Fourth Wall (AFL 4) is behind the watermark")
	assert.True(t, f.monitored[9007], "Fifth Gear (AFL 5) is ahead")
}

func TestEpisodesAFLDoesNotCountAreNeverHeldForAFL(t *testing.T) {
	// AFL will never list a banned episode, so waiting for it would hold
	// the episode forever.
	e, f, id := newBannedEngine(t, "listed", time.Now().Add(-24*time.Hour), 0)
	resync(t, e, id)

	assert.Empty(t, heldIDs(t, e, id))
	assert.True(t, f.monitored[9004])
	assert.True(t, f.monitored[9005])
}

func TestAutoWatermarkIgnoresEpisodesAFLDoesNotCount(t *testing.T) {
	aired := time.Now().AddDate(-1, 0, 0)
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for i, title := range bannedTitles {
		n := i + 1
		downloaded := n <= 3 || title == "Banned B"
		sonarr = append(sonarr, models.SonarrEpisode{ID: 9000 + n, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: &n, Title: title, HasFile: downloaded, AirDateUTC: aired})
		if !strings.HasPrefix(title, "Banned") {
			afl = append(afl, models.AFLEpisode{Number: len(afl) + 1, Title: title, Type: models.EpisodeCanon})
		}
	}
	// Banned B comes before AFL 4, so having it says no more than AFL 3.
	assert.Equal(t, 3, initialWatermarkForEpisodes(Place(sonarr, afl), time.Now()))
}
