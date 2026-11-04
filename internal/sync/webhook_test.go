package sync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	stdsync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	"mizu/internal/sonarr"
)

const webhookAFLHTML = `<!DOCTYPE html><html><body>
<span property="dc:title" content="Test Show"></span>
<table class="EpisodeList"><tbody>
<tr><td class="Number">1</td><td class="Title"><a href="#">Episode 1</a></td><td class="Type"><span>filler</span></td></tr>
</tbody></table>
</body></html>`

// webhookSonarr is a Sonarr stub with one episode, a download queue and
// mutable monitored state.
type webhookSonarr struct {
	mu        stdsync.Mutex
	monitored bool
	queue     []sonarr.QueueItem
	removed   []string
	reads     int // episode list fetches
}

func (f *webhookSonarr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	abs := 1
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series/100":
		_ = json.NewEncoder(w).Encode(models.SonarrSeries{ID: 100, Title: "Test Show"})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/episode":
		f.reads++
		_ = json.NewEncoder(w).Encode([]models.SonarrEpisode{{
			ID: 9001, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: 1, AbsoluteEpisodeNumber: &abs,
			Title: "Episode 1", Monitored: f.monitored, AirDateUTC: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}})
	case r.Method == http.MethodPut && r.URL.Path == "/api/v3/episode/monitor":
		var body struct {
			Monitored bool `json:"monitored"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.monitored = body.Monitored
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/queue/details":
		_ = json.NewEncoder(w).Encode(f.queue)
	case r.Method == http.MethodDelete && len(r.URL.Path) > len("/api/v3/queue/"):
		f.removed = append(f.removed, r.URL.RequestURI())
	default:
		http.NotFound(w, r)
	}
}

func newWebhookEngine(t *testing.T, monitored bool) (*Engine, *webhookSonarr, int64) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	fake := &webhookSonarr{monitored: monitored, queue: []sonarr.QueueItem{{ID: 7, EpisodeID: 9001, DownloadID: "ABC123"}}}
	sonarrSrv := httptest.NewServer(fake)
	t.Cleanup(sonarrSrv.Close)
	aflSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, webhookAFLHTML)
	}))
	t.Cleanup(aflSrv.Close)

	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarrSrv.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 5 * time.Second
	e := New(cfg, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)

	m := &models.ShowMapping{
		SonarrSeriesID:   100,
		SonarrTitle:      "Test Show",
		AFLSlug:          "test-show",
		AFLTitle:         "Test Show",
		Enabled:          true,
		FillerAction:     models.ActionUnmonitor,
		MixedAction:      models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor,
		StartMode:        models.StartModeFromBeginning,
		WatermarkMode:    models.WatermarkModeManual,
	}
	require.NoError(t, db.UpsertMapping(context.Background(), m))

	saved := queuePollDelays
	queuePollDelays = []time.Duration{0}
	t.Cleanup(func() { queuePollDelays = saved })
	return e, fake, m.ID
}

func lastWebhookItem(t *testing.T, e *Engine) (models.SyncLog, models.SyncLogItem) {
	t.Helper()
	page, err := e.db.GetSyncLogsPage(context.Background(), 1, 0)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	lg := page.Items[0]
	items, err := e.db.GetSyncLogItems(context.Background(), []int64{lg.ID})
	require.NoError(t, err)
	require.Len(t, items[lg.ID], 1)
	return lg, items[lg.ID][0]
}

// Sonarr grabbed a filler that was still monitored: the resync unmonitors
// it and the download is cancelled.
func TestWebhookCancelsFillerGrab(t *testing.T) {
	e, fake, id := newWebhookEngine(t, true)

	require.NoError(t, e.RunWebhook(context.Background(), id, []int{9001}, "abc123"))

	assert.False(t, fake.monitored)
	assert.Equal(t, []string{"/api/v3/queue/7?removeFromClient=true&blocklist=false"}, fake.removed)
	lg, item := lastWebhookItem(t, e)
	assert.Equal(t, "webhook", lg.Trigger)
	assert.Equal(t, "success", lg.Status)
	assert.Contains(t, item.Summary, "cancelled the download")
}

// An episode that was already unmonitored was grabbed by hand; Mizu keeps
// its hands off the download.
func TestWebhookLeavesManualGrabAlone(t *testing.T) {
	e, fake, id := newWebhookEngine(t, false)

	require.NoError(t, e.RunWebhook(context.Background(), id, []int{9001}, "abc123"))

	assert.Empty(t, fake.removed)
	_, item := lastWebhookItem(t, e)
	assert.NotContains(t, item.Summary, "download")
}

func TestWebhookWaitsForRunningSync(t *testing.T) {
	e, fake, id := newWebhookEngine(t, true)
	done, ok := e.TryAcquireRun()
	require.True(t, ok)

	errCh := make(chan error, 1)
	go func() { errCh <- e.RunWebhook(context.Background(), id, nil, "") }()
	select {
	case <-errCh:
		t.Fatal("webhook ran while another sync held the slot")
	case <-time.After(100 * time.Millisecond):
	}
	done()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("webhook never ran")
	}
	assert.False(t, fake.monitored)
	assert.Empty(t, fake.removed, "a series add cancels nothing")
}

// Sonarr grabbed a filler while it was monitored, but a sync running when
// the webhook arrived unmonitored it first. The grab still wasn't made by
// hand, so the download is cancelled.
func TestWebhookCancelsAGrabUnmonitoredWhileItWaits(t *testing.T) {
	e, fake, id := newWebhookEngine(t, true)
	done, ok := e.TryAcquireRun()
	require.True(t, ok)

	errCh := make(chan error, 1)
	go func() { errCh <- e.RunWebhook(context.Background(), id, []int{9001}, "abc123") }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		fake.mu.Lock()
		read := fake.reads > 0
		fake.mu.Unlock()
		if read {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	fake.mu.Lock()
	fake.monitored = false // the running sync's doing
	fake.mu.Unlock()
	done()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("webhook never ran")
	}
	assert.Equal(t, []string{"/api/v3/queue/7?removeFromClient=true&blocklist=false"}, fake.removed)
}
