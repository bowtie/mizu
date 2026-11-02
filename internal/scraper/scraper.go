package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"mizu/internal/database"
	"mizu/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

const (
	baseURL                 = "https://www.animefillerlist.com"
	userAgent               = "Mizu/1.0"
	showListRefreshInterval = 30 * 24 * time.Hour

	// maxResponseBytes caps how much HTML is parsed per page. AFL pages
	// are well under 1 MB; anything larger means a hostile or broken
	// mirror, and parsing it unbounded could exhaust memory.
	maxResponseBytes = 8 << 20 // 8 MiB
)

type Scraper struct {
	client         *http.Client
	rateLimitDelay time.Duration
	lastRequest    time.Time
	mu             sync.Mutex
	db             *database.DB
	baseURL        string

	// showListFlight coalesces concurrent show-list scrapes (the
	// GetAllShows stale-cache fallback and the manual refresh handler) onto
	// a single in-flight fetch instead of double-scraping
	// animefillerlist.com. The cache holds no per-key user state, so reuse
	// of the in-flight result is always safe.
	showListFlight singleflight.Group

	// onNewShows, when set, is called once per scrape that discovered
	// shows missing from the previous cache. It runs for every refresh
	// path (manual button, stale-cache fallback) because it hangs off the
	// scrape itself rather than a particular caller.
	onNewShows func(ctx context.Context, shows []models.AFLShow)
}

// SetOnNewShows registers the callback invoked with newly discovered shows.
func (s *Scraper) SetOnNewShows(fn func(ctx context.Context, shows []models.AFLShow)) {
	s.onNewShows = fn
}

func New(requestTimeout, rateLimitDelay time.Duration, db *database.DB) *Scraper {
	return &Scraper{
		client:         &http.Client{Timeout: requestTimeout},
		rateLimitDelay: rateLimitDelay,
		db:             db,
		baseURL:        baseURL,
	}
}

// SetBaseURL overrides the AFL base URL. Tests use this to point the
// scraper at a local stub instead of animefillerlist.com; production always
// uses the package-level default.
func (s *Scraper) SetBaseURL(u string) {
	s.baseURL = u
}

func (s *Scraper) rateLimit(ctx context.Context) error {
	s.mu.Lock()
	since := time.Since(s.lastRequest)
	if since >= s.rateLimitDelay {
		// Enough time has passed — claim this slot immediately.
		s.lastRequest = time.Now()
		s.mu.Unlock()
		return nil
	}
	// Not enough time yet — reserve our slot now so concurrent callers
	// see a future lastRequest and wait longer. This prevents two
	// goroutines from both passing the check and firing simultaneously.
	wait := s.rateLimitDelay - since
	s.lastRequest = s.lastRequest.Add(s.rateLimitDelay)
	s.mu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	return nil
}

func (s *Scraper) fetch(ctx context.Context, url string) (*goquery.Document, error) {
	if err := s.rateLimit(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML from %s: %w", url, err)
	}

	return doc, nil
}

func (s *Scraper) GetAllShows(ctx context.Context) ([]models.AFLShow, error) {
	if s.db != nil {
		cached, refreshedAt, err := s.db.GetCachedAFLShows(ctx)
		if err != nil {
			return nil, err
		}
		if len(cached) > 0 && !refreshedAt.IsZero() && time.Since(refreshedAt) < showListRefreshInterval {
			return cached, nil
		}
	}

	shows, _, err := s.RefreshAllShows(ctx)
	if err != nil && s.db != nil {
		// The refresh failed (AFL down, redesigned, rate-limiting us...).
		// Fall back to the stale cache rather than failing auto-match
		// outright — an out-of-date show list beats no show list.
		stale, refreshedAt, cacheErr := s.db.GetCachedAFLShows(ctx)
		if cacheErr == nil && len(stale) > 0 {
			log.Warn().Err(err).
				Int("count", len(stale)).
				Time("cache_refreshed_at", refreshedAt).
				Msg("AFL refresh failed; serving stale cache")
			return stale, nil
		}
	}
	return shows, err
}

func (s *Scraper) RefreshAllShows(ctx context.Context) ([]models.AFLShow, []string, error) {
	// Coalesce concurrent refreshes onto one in-flight scrape: a
	// create-with-auto-match hitting a stale cache while a manual refresh
	// is in flight (or two stale-cache fallbacks at once) awaits the same
	// result instead of scraping twice. If the leader's scrape fails, all
	// waiters inherit its error rather than re-scraping — the deliberate
	// singleflight tradeoff (retrying on a shared failure would reintroduce
	// the double-scrape).
	type refreshResult struct {
		shows    []models.AFLShow
		newSlugs []string
	}
	res, err, _ := s.showListFlight.Do("show-list", func() (interface{}, error) {
		shows, newSlugs, err := s.doRefreshAllShows(ctx)
		if err != nil {
			return nil, err
		}
		return refreshResult{shows: shows, newSlugs: newSlugs}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	r := res.(refreshResult)
	return r.shows, r.newSlugs, nil
}

// doRefreshAllShows performs the actual scrape and cache write, running
// under the singleflight gate in RefreshAllShows.
func (s *Scraper) doRefreshAllShows(ctx context.Context) ([]models.AFLShow, []string, error) {
	log.Info().Msg("fetching show list from AnimeFillerList")

	doc, err := s.fetch(ctx, s.baseURL+"/shows")
	if err != nil {
		return nil, nil, fmt.Errorf("fetching show list: %w", err)
	}

	var shows []models.AFLShow
	doc.Find("#ShowList .Group ul li a").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}

		title := strings.TrimSpace(sel.Text())
		slug := extractSlug(href)
		if title != "" && slug != "" {
			shows = append(shows, models.AFLShow{Title: title, Slug: slug, URL: s.baseURL + href})
		}
	})

	// A zero-count parse means the page no longer matches the expected
	// structure (redesign, bot-block interstitial) — treat it as an error
	// instead of writing an empty cache that would wipe coverage.
	if len(shows) == 0 {
		return nil, nil, fmt.Errorf("parsed 0 shows from %s — AnimeFillerList layout may have changed", s.baseURL+"/shows")
	}

	log.Info().Int("count", len(shows)).Msg("fetched show list")

	var newSlugs []string
	if s.db != nil {
		newSlugs, err = s.db.ReplaceCachedAFLShows(ctx, shows, time.Now().UTC())
		if err != nil {
			return nil, nil, err
		}
		if len(newSlugs) > 0 {
			log.Info().Int("new", len(newSlugs)).Msg("new shows detected in AFL cache")
		}
		// Every show being "new" means the cache was empty (fresh install,
		// restore): that is a first fill, not a discovery worth announcing.
		if s.onNewShows != nil && len(newSlugs) > 0 && len(newSlugs) < len(shows) {
			s.onNewShows(ctx, showsBySlug(shows, newSlugs))
		}
	}

	return shows, newSlugs, nil
}

// showsBySlug returns the shows whose slugs appear in slugs, in slugs order.
func showsBySlug(shows []models.AFLShow, slugs []string) []models.AFLShow {
	bySlug := make(map[string]models.AFLShow, len(shows))
	for _, sh := range shows {
		bySlug[sh.Slug] = sh
	}
	out := make([]models.AFLShow, 0, len(slugs))
	for _, slug := range slugs {
		if sh, ok := bySlug[slug]; ok {
			out = append(out, sh)
		}
	}
	return out
}

func (s *Scraper) GetShowEpisodes(ctx context.Context, slug string) (string, []models.AFLEpisode, error) {
	// Validate slug to prevent path traversal in the URL.
	for _, ch := range slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return "", nil, fmt.Errorf("invalid slug: %s", slug)
		}
	}
	url := fmt.Sprintf("%s/shows/%s", s.baseURL, slug)
	log.Info().Str("slug", slug).Str("url", url).Msg("fetching episode data")

	doc, err := s.fetch(ctx, url)
	if err != nil {
		return "", nil, fmt.Errorf("fetching episodes for %s: %w", slug, err)
	}

	showTitle := ""
	doc.Find("span[property='dc:title']").Each(func(i int, sel *goquery.Selection) {
		if content, exists := sel.Attr("content"); exists {
			showTitle = strings.TrimSpace(content)
		}
	})
	if showTitle == "" {
		showTitle = strings.TrimSpace(doc.Find("h1").First().Text())
		showTitle = strings.TrimSuffix(showTitle, " Filler List")
	}

	var episodes []models.AFLEpisode
	doc.Find("table.EpisodeList tbody tr").Each(func(i int, sel *goquery.Selection) {
		numStr := strings.TrimSpace(sel.Find("td.Number").Text())
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return
		}

		title := strings.TrimSpace(sel.Find("td.Title a").Text())
		if title == "" {
			title = strings.TrimSpace(sel.Find("td.Title").Text())
		}

		typeStr := strings.TrimSpace(sel.Find("td.Type span").Text())
		epType := parseEpisodeType(typeStr)
		episodes = append(episodes, models.AFLEpisode{Number: num, Title: title, Type: epType})
	})

	// A missing EpisodeList table means structural breakage (redesign,
	// bot-block) — error so sync marks the show failed and the stored
	// episode types survive. A table that parses to zero rows is left
	// alone; that's unusual but not structurally impossible.
	if doc.Find("table.EpisodeList").Length() == 0 {
		return "", nil, fmt.Errorf("episode list table not found for %s — AnimeFillerList layout may have changed", slug)
	}

	log.Info().Str("show", showTitle).Int("episodes", len(episodes)).Msg("parsed episode data")
	return showTitle, episodes, nil
}

func parseEpisodeType(typeStr string) models.EpisodeType {
	normalized := strings.ToLower(strings.TrimSpace(typeStr))
	normalized = strings.ReplaceAll(normalized, "\u00a0", " ")
	normalized = strings.Join(strings.Fields(normalized), " ")

	switch {
	case strings.Contains(normalized, "manga canon") || normalized == "canon":
		return models.EpisodeCanon
	case strings.Contains(normalized, "anime canon"):
		return models.EpisodeAnimeCanon
	case strings.Contains(normalized, "mixed"):
		return models.EpisodeMixedCanon
	case strings.Contains(normalized, "filler"):
		return models.EpisodeFiller
	default:
		log.Warn().Str("type", typeStr).Msg("unknown episode type")
		return models.EpisodeUnknown
	}
}

func extractSlug(href string) string {
	href = strings.TrimPrefix(href, "/shows/")
	href = strings.TrimSuffix(href, "/")
	return href
}
