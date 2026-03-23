package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
	"mizu/internal/database"
	"mizu/internal/models"
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
}

func New(requestTimeout, rateLimitDelay time.Duration, db *database.DB) *Scraper {
	return &Scraper{
		client:         &http.Client{Timeout: requestTimeout},
		rateLimitDelay: rateLimitDelay,
		db:             db,
	}
}

func (s *Scraper) rateLimit() {
	s.mu.Lock()
	defer s.mu.Unlock()

	since := time.Since(s.lastRequest)
	if since < s.rateLimitDelay {
		time.Sleep(s.rateLimitDelay - since)
	}
	s.lastRequest = time.Now()
}

func (s *Scraper) fetch(ctx context.Context, url string) (*goquery.Document, error) {
	s.rateLimit()

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

	return s.RefreshAllShows(ctx)
}

func (s *Scraper) RefreshAllShows(ctx context.Context) ([]models.AFLShow, error) {
	log.Info().Msg("fetching show list from AnimeFillerList")

	doc, err := s.fetch(ctx, baseURL+"/shows")
	if err != nil {
		return nil, fmt.Errorf("fetching show list: %w", err)
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
			shows = append(shows, models.AFLShow{Title: title, Slug: slug, URL: baseURL + href})
		}
	})

	log.Info().Int("count", len(shows)).Msg("fetched show list")

	if s.db != nil {
		if err := s.db.ReplaceCachedAFLShows(ctx, shows, time.Now().UTC()); err != nil {
			return nil, err
		}
	}

	return shows, nil
}

func (s *Scraper) SearchShows(ctx context.Context, query string) ([]models.AFLShow, error) {
	allShows, err := s.GetAllShows(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []models.AFLShow
	for _, show := range allShows {
		if strings.Contains(strings.ToLower(show.Title), query) {
			results = append(results, show)
		}
	}
	return results, nil
}

func (s *Scraper) GetShowEpisodes(ctx context.Context, slug string) (string, []models.AFLEpisode, error) {
	url := fmt.Sprintf("%s/shows/%s", baseURL, slug)
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
