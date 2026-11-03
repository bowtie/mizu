package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	"mizu/internal/models"
)

// newTestScraper builds a Scraper pointed at srv (which may be nil) with
// rate limiting disabled, backed by a temp database.
func newTestScraper(t *testing.T, srv *httptest.Server) *Scraper {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	s := New(2*time.Second, 0, db)
	if srv != nil {
		s.SetBaseURL(srv.URL)
	}
	return s
}

const validShowListHTML = `<!DOCTYPE html><html><body>
<div id="ShowList">
  <div class="Group"><ul>
    <li><a href="/shows/one-piece">One Piece</a></li>
    <li><a href="/shows/bleach">Bleach</a></li>
  </ul></div>
</div>
</body></html>`

func TestRefreshAllShowsRejectsRedesignedLayout(t *testing.T) {
	// A page that no longer contains the expected selectors must be an
	// error — writing an empty cache would wipe coverage.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body><p>Welcome to our new site!</p></body></html>"))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	shows, _, err := s.RefreshAllShows(context.Background())
	require.Error(t, err)
	assert.Nil(t, shows)
	assert.Contains(t, err.Error(), "parsed 0 shows")

	// The cache must be untouched (still empty here, but no bogus write).
	count, _, err := s.db.GetAFLShowCacheStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestGetShowEpisodesRejectsMissingEpisodeTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body><h1>Access denied</h1></body></html>"))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	title, episodes, err := s.GetShowEpisodes(context.Background(), "one-piece")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "episode list table not found")
	assert.Empty(t, title)
	assert.Nil(t, episodes)
}

func TestGetShowEpisodesAcceptsEmptyTableRows(t *testing.T) {
	// Table present but zero rows is unusual yet structurally valid.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
		<span property="dc:title" content="Empty Show"></span>
		<table class="EpisodeList"><tbody></tbody></table>
		</body></html>`))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	title, episodes, err := s.GetShowEpisodes(context.Background(), "empty-show")
	require.NoError(t, err)
	assert.Equal(t, "Empty Show", title)
	assert.Empty(t, episodes)
}

func TestGetAllShowsFallsBackToStaleCacheWhenAFLFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	ctx := context.Background()

	// Seed a stale cache entry (refreshed long ago so the fresh path skips).
	_, err := s.db.ReplaceCachedAFLShows(ctx, []models.AFLShow{
		{Title: "Old Show", Slug: "old-show", URL: "https://www.animefillerlist.com/shows/old-show"},
	}, time.Now().Add(-90*24*time.Hour))
	require.NoError(t, err)

	shows, err := s.GetAllShows(ctx)
	require.NoError(t, err, "stale cache must be served when the refresh fails")
	require.Len(t, shows, 1)
	assert.Equal(t, "old-show", shows[0].Slug)
}

func TestGetAllShowsErrorsWithoutCacheWhenAFLFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	_, err := s.GetAllShows(context.Background())
	require.Error(t, err, "with no cache to fall back on, the failure must propagate")
}

func TestFetchCapsResponseSize(t *testing.T) {
	// A body larger than maxResponseBytes should truncate rather than
	// exhaust memory; the truncated page fails the structure checks.
	big := make([]byte, maxResponseBytes+1024)
	for i := range big {
		big[i] = 'x'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	_, _, err := s.RefreshAllShows(context.Background())
	require.Error(t, err, "oversized response must not parse as a valid show list")
}

func TestGetAllShowsUsesFreshCacheWithoutHittingAFL(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(validShowListHTML))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	ctx := context.Background()

	_, err := s.db.ReplaceCachedAFLShows(ctx, []models.AFLShow{
		{Title: "Cached Show", Slug: "cached-show"},
	}, time.Now().Add(-time.Hour)) // well within the 30-day window
	require.NoError(t, err)

	shows, err := s.GetAllShows(ctx)
	require.NoError(t, err)
	require.Len(t, shows, 1)
	assert.Equal(t, 0, hits, "fresh cache must not trigger a scrape")
}

func showListHTML(slugs ...string) string {
	html := `<!DOCTYPE html><html><body><div id="ShowList"><div class="Group"><ul>`
	for _, s := range slugs {
		html += `<li><a href="/shows/` + s + `">` + s + `</a></li>`
	}
	return html + `</ul></div></div></body></html>`
}

func TestNewShowsHookSkipsFirstFillAndReportsLaterAdditions(t *testing.T) {
	page := showListHTML("one-piece", "bleach")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	var got [][]models.AFLShow
	s.SetOnNewShows(func(_ context.Context, shows []models.AFLShow) { got = append(got, shows) })

	// Empty cache: everything is "new", which is a first fill, not news.
	_, _, err := s.RefreshAllShows(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)

	// Same list again: nothing new.
	_, _, err = s.RefreshAllShows(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)

	// A show appears upstream: the hook gets exactly that one.
	page = showListHTML("one-piece", "bleach", "frieren")
	_, _, err = s.RefreshAllShows(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0], 1)
	assert.Equal(t, "frieren", got[0][0].Slug)
}

func TestGetShowEpisodesReadsAirDates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
		<span property="dc:title" content="Naruto"></span>
		<table class="EpisodeList"><tbody>
		<tr><td class="Number">1</td><td class="Title"><a href="#">Enter: Naruto Uzumaki!</a></td><td class="Type"><span>Manga Canon</span></td><td class="Date">2002-10-03</td></tr>
		<tr><td class="Number">2</td><td class="Title"><a href="#">My Name is Konohamaru!</a></td><td class="Type"><span>Manga Canon</span></td><td class="Date"></td></tr>
		</tbody></table>
		</body></html>`))
	}))
	defer srv.Close()

	s := newTestScraper(t, srv)
	_, episodes, err := s.GetShowEpisodes(context.Background(), "naruto")
	require.NoError(t, err)
	require.Len(t, episodes, 2)
	assert.Equal(t, time.Date(2002, 10, 3, 0, 0, 0, 0, time.UTC), episodes[0].AirDate)
	assert.True(t, episodes[1].AirDate.IsZero(), "a missing date is left empty")
}
