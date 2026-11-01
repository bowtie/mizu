package scraper

import (
	"context"
	"fmt"
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

	doc, err := goquery.NewDocumentFromReader(resp.Body)
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
	}

	return shows, newSlugs, nil
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
