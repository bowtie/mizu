package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	syncengine "mizu/internal/sync"
)

// Shutdown closes the database once the background wait group is done, so
// that wait has to cover a scheduled sync still talking to Sonarr.
func TestShutdownWaitsForAScheduledSync(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.UpsertMapping(context.Background(), &models.ShowMapping{
		SonarrSeriesID: 100, SonarrTitle: "Test Show", AFLSlug: "test-show", Enabled: true,
	}))

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	sonarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(sonarr.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarr.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 30 * time.Second
	engine := syncengine.New(cfg, db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var bgWG sync.WaitGroup
	startBackground(ctx, cfg, newSchedulerController(20*time.Millisecond), engine, db, &bgWG)

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the scheduled sync never reached Sonarr")
	}
	waited := make(chan struct{})
	go func() {
		bgWG.Wait()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("the shutdown wait returned while a scheduled sync was talking to Sonarr")
	case <-time.After(200 * time.Millisecond):
	}

	cancel()
	close(release)
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("the background loops didn't stop after cancel")
	}
	done, ok := engine.TryAcquireRun()
	require.True(t, ok, "no sync is running once the wait returns")
	done()
}
