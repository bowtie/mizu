package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	"mizu/internal/models"
)

// staleShowListHTML matches the #ShowList .Group ul li a shape the scraper
// parses, with one show.
const staleShowListHTML = `<!DOCTYPE html><html><body>
<div id="ShowList"><div class="Group"><ul>
<li><a href="/shows/naruto">Naruto</a></li>
</ul></div></div>
</body></html>`

// countingShowListStub serves the show list and counts scrapes. With
// blockFirst it parks the first request (so a concurrent caller's reuse of
// the in-flight scrape is observable) and serves later requests immediately.
type countingShowListStub struct {
	hits       atomic.Int32
	blockFirst bool
	// blocked is written by the first request before closing entered and
	// read by any later request only after the test has received entered
	// (close/receive/goroutine-start happens-before chain), so it needs no
	// mutex.
	blocked bool
	entered chan struct{}
	release chan struct{}
}

func (f *countingShowListStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.hits.Add(1)
	if f.blockFirst && !f.blocked {
		f.blocked = true
		close(f.entered)
		<-f.release
	}
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(staleShowListHTML))
}

// seedStaleShowCache writes a show list whose refreshed_at is older than the
// 30-day freshness window, so GetAllShows falls through to a scrape.
func seedStaleShowCache(t *testing.T, db *database.DB) {
	t.Helper()
	_, err := db.ReplaceCachedAFLShows(context.Background(), []models.AFLShow{
		{Title: "Old List", Slug: "old", URL: "https://www.animefillerlist.com/shows/old"},
	}, time.Now().Add(-31*24*time.Hour))
	require.NoError(t, err)
}

// waitScraperDone blocks until the goroutine finished, failing on timeout.
func waitScraperDone(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// TestGetAllShowsCoalescesStaleCacheRefreshes is the failing-first
// reproduction of the un-gated fallback scrape: two concurrent GetAllShows
// calls with a stale cache must perform exactly one scrape of the AFL site —
// the second call must await the first's in-flight refresh rather than
// scraping again (which would double-scrape animefillerlist.com and
// double-log activity-feed entries).
func TestGetAllShowsCoalescesStaleCacheRefreshes(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	stub := &countingShowListStub{blockFirst: true, entered: make(chan struct{}), release: make(chan struct{})}
	srv := httptest.NewServer(stub)
	defer srv.Close()

	s := New(5*time.Second, 0, db)
	s.SetBaseURL(srv.URL)
	seedStaleShowCache(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	firstDone := make(chan struct{})
	var firstShows []models.AFLShow
	var firstErr error
	go func() {
		defer close(firstDone)
		firstShows, firstErr = s.GetAllShows(ctx)
	}()
	<-stub.entered // first scrape is in flight (parked in the stub)

	secondDone := make(chan struct{})
	var secondShows []models.AFLShow
	var secondErr error
	go func() {
		defer close(secondDone)
		secondShows, secondErr = s.GetAllShows(ctx)
	}()

	// Pre-fix, the second call scrapes independently and returns promptly;
	// post-fix it awaits the first's in-flight scrape, so it stays pending.
	select {
	case <-secondDone:
		// Pre-fix: second call already scraped on its own.
	case <-time.After(500 * time.Millisecond):
		// Post-fix: second call is awaiting the in-flight refresh.
	}

	// Exactly one scrape must have happened so far.
	assert.Equal(t, int32(1), stub.hits.Load(), "concurrent GetAllShows with a stale cache must perform exactly one scrape")

	close(stub.release)
	waitScraperDone(t, firstDone, "first GetAllShows")
	waitScraperDone(t, secondDone, "second GetAllShows")
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Len(t, firstShows, 1)
	require.Len(t, secondShows, 1)
	assert.Equal(t, "Naruto", firstShows[0].Title)
	assert.Equal(t, "Naruto", secondShows[0].Title)
	assert.Equal(t, int32(1), stub.hits.Load(), "still exactly one scrape after both calls complete")
}
