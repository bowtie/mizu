package artwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"mizu/internal/models"
)

// KitsuProvider fetches artwork from Kitsu.io's public API.
// No API key required.
type KitsuProvider struct {
	httpClient *http.Client
}

func NewKitsuProvider(timeout time.Duration) *KitsuProvider {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &KitsuProvider{httpClient: &http.Client{Timeout: timeout}}
}

func (p *KitsuProvider) Name() string    { return "kitsu" }
func (p *KitsuProvider) Available() bool { return true }

type kitsuResp struct {
	Data []struct {
		Attributes struct {
			Slug           string            `json:"slug"`
			CanonicalTitle string            `json:"canonicalTitle"`
			Titles         map[string]string `json:"titles"`
			PosterImage    struct {
				Original string `json:"original"`
				Large    string `json:"large"`
				Medium   string `json:"medium"`
			} `json:"posterImage"`
			CoverImage struct {
				Original string `json:"original"`
				Large    string `json:"large"`
			} `json:"coverImage"`
		} `json:"attributes"`
	} `json:"data"`
}

func (p *KitsuProvider) Fetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	client := p.httpClient
	titles := searchTitles(series)

	var best *Artwork
	var bestConf float64

	for _, title := range titles {
		u := fmt.Sprintf("https://kitsu.io/api/edge/anime?filter[text]=%s&page[limit]=5",
			url.QueryEscape(title))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/vnd.api+json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var decoded kitsuResp
		err = json.NewDecoder(resp.Body).Decode(&decoded)
		resp.Body.Close()
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || len(decoded.Data) == 0 {
			continue
		}

		for _, item := range decoded.Data {
			attr := item.Attributes
			candidates := uniqueStrings(
				attr.CanonicalTitle,
				attr.Titles["en"],
				attr.Titles["en_jp"],
				attr.Titles["ja_jp"],
				attr.Slug,
			)
			conf, _ := bestTitleMatch(titles, candidates)
			if conf < 0.80 || conf <= bestConf {
				continue
			}

			poster := firstNonEmpty(attr.PosterImage.Original, attr.PosterImage.Large)
			banner := firstNonEmpty(attr.CoverImage.Original, attr.CoverImage.Large)
			if poster == "" && banner == "" {
				continue
			}

			best = &Artwork{
				PosterURL: poster,
				BannerURL: banner,
				Source:    "kitsu",
			}
			bestConf = conf
			if bestConf >= 0.96 {
				return best, nil
			}
		}
	}

	return best, nil
}
