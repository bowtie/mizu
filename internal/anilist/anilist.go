package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
)

const endpoint = "https://graphql.anilist.co"

const searchQuery = `
query ($search: String!) {
  Page(page: 1, perPage: 8) {
    media(search: $search, type: ANIME, sort: SEARCH_MATCH) {
      title {
        romaji
        english
        native
        userPreferred
      }
      bannerImage
      coverImage {
        extraLarge
        large
        medium
        color
      }
    }
  }
}`

type Client struct {
	endpoint   string
	httpClient *http.Client
}

type MediaTitle struct {
	Romaji        string `json:"romaji"`
	English       string `json:"english"`
	Native        string `json:"native"`
	UserPreferred string `json:"userPreferred"`
}

type MediaCoverImage struct {
	ExtraLarge string `json:"extraLarge"`
	Large      string `json:"large"`
	Medium     string `json:"medium"`
	Color      string `json:"color"`
}

type Media struct {
	Title       MediaTitle      `json:"title"`
	BannerImage string          `json:"bannerImage"`
	CoverImage  MediaCoverImage `json:"coverImage"`
}

type Artwork struct {
	BannerImage     string  `json:"banner_image,omitempty"`
	PosterImage     string  `json:"poster_image,omitempty"`
	Color           string  `json:"color,omitempty"`
	MatchTitle      string  `json:"match_title,omitempty"`
	MatchConfidence float64 `json:"match_confidence,omitempty"`
	Source          string  `json:"source"`
}

type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse struct {
	Data struct {
		Page struct {
			Media []Media `json:"media"`
		} `json:"page"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) SearchArtwork(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	var (
		best           *Artwork
		bestConfidence float64
		successful     bool
		firstErr       error
	)

	for _, title := range searchTitles(series) {
		media, err := c.searchMedia(ctx, title)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		successful = true

		match := BestArtworkMatch(ctx, series, media)
		if match == nil || match.MatchConfidence <= bestConfidence {
			continue
		}

		best = match
		bestConfidence = match.MatchConfidence
		if bestConfidence >= 0.96 {
			break
		}
	}

	if best != nil {
		return best, nil
	}
	if successful {
		return nil, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func (c *Client) searchMedia(ctx context.Context, search string) ([]Media, error) {
	payload, err := json.Marshal(graphQLRequest{
		Query: searchQuery,
		Variables: map[string]interface{}{
			"search": search,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal AniList query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create AniList request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AniList request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AniList returned %d", resp.StatusCode)
	}

	var decoded graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode AniList response: %w", err)
	}

	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("AniList GraphQL error: %s", decoded.Errors[0].Message)
	}

	return decoded.Data.Page.Media, nil
}

func BestArtworkMatch(_ context.Context, series models.SonarrSeries, results []Media) *Artwork {
	matcher := matcherpkg.New(0.88)
	seriesTitles := searchTitles(series)

	var (
		best           *Artwork
		bestConfidence float64
	)

	for _, media := range results {
		banner := strings.TrimSpace(media.BannerImage)
		poster := firstNonEmpty(media.CoverImage.ExtraLarge, media.CoverImage.Large, media.CoverImage.Medium)
		if banner == "" && poster == "" {
			continue
		}

		confidence, matchedTitle := bestTitleConfidence(matcher, seriesTitles, media.titleCandidates())
		if confidence < 0.88 || confidence <= bestConfidence {
			continue
		}

		best = &Artwork{
			BannerImage:     banner,
			PosterImage:     poster,
			Color:           strings.TrimSpace(media.CoverImage.Color),
			MatchTitle:      matchedTitle,
			MatchConfidence: confidence,
			Source:          "anilist",
		}
		bestConfidence = confidence
	}

	return best
}

func (m Media) titleCandidates() []string {
	return uniqueStrings(
		m.Title.UserPreferred,
		m.Title.English,
		m.Title.Romaji,
		m.Title.Native,
	)
}

func searchTitles(series models.SonarrSeries) []string {
	titles := []string{
		series.Title,
		series.SortTitle,
	}
	for _, alt := range series.AlternateTitles {
		titles = append(titles, alt.Title)
	}
	return uniqueStrings(titles...)
}

func bestTitleConfidence(m *matcherpkg.Matcher, seriesTitles, mediaTitles []string) (float64, string) {
	bestConfidence := 0.0
	bestTitle := ""

	for _, seriesTitle := range seriesTitles {
		for _, mediaTitle := range mediaTitles {
			confidence := titleConfidence(m, seriesTitle, mediaTitle)
			if confidence > bestConfidence {
				bestConfidence = confidence
				bestTitle = mediaTitle
			}
		}
	}

	return bestConfidence, bestTitle
}

func titleConfidence(m *matcherpkg.Matcher, seriesTitle, mediaTitle string) float64 {
	if strings.EqualFold(seriesTitle, mediaTitle) {
		return 1.0
	}

	normSeries := m.Normalize(seriesTitle)
	normMedia := m.Normalize(mediaTitle)
	if normSeries == "" || normMedia == "" {
		return 0
	}
	if normSeries == normMedia {
		return 0.96
	}
	if matcherpkg.SharedTokenCount(normSeries, normMedia) == 0 {
		return 0
	}

	confidence := matcherpkg.ContainmentScore(normSeries, normMedia)
	if jw := matcherpkg.JaroWinkler(normSeries, normMedia); jw >= 0.9 && jw > confidence {
		confidence = jw
	}
	return confidence
}

func uniqueStrings(values ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
