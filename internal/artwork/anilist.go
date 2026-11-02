package artwork

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"mizu/internal/models"
	"mizu/internal/netguard"
)

// AniListProvider fetches artwork from AniList's public GraphQL API.
// No API key required.
type AniListProvider struct {
	httpClient *http.Client
}

func NewAniListProvider(timeout time.Duration) *AniListProvider {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &AniListProvider{httpClient: netguard.NewClient(timeout, false)}
}

func (p *AniListProvider) Name() string    { return "anilist" }
func (p *AniListProvider) Available() bool { return true }

const anilistQuery = `
query ($search: String!) {
  Page(page: 1, perPage: 8) {
    media(search: $search, type: ANIME, sort: SEARCH_MATCH) {
      title { romaji english native userPreferred }
      bannerImage
      coverImage { extraLarge large medium color }
    }
  }
}`

type anilistResp struct {
	Data struct {
		Page struct {
			Media []struct {
				Title struct {
					Romaji, English, Native, UserPreferred string
				}
				BannerImage string
				CoverImage  struct {
					ExtraLarge, Large, Medium, Color string
				}
			}
		}
	} `json:"data"`
	Errors []struct{ Message string } `json:"errors"`
}

func (p *AniListProvider) Fetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	client := p.httpClient
	titles := searchTitles(series)
	seriesTitles := titles

	var best *Artwork
	var bestConf float64

	for _, title := range titles {
		payload, _ := json.Marshal(map[string]interface{}{
			"query":     anilistQuery,
			"variables": map[string]interface{}{"search": title},
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graphql.anilist.co", bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var decoded anilistResp
		err = json.NewDecoder(resp.Body).Decode(&decoded)
		resp.Body.Close()
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || len(decoded.Errors) > 0 {
			continue
		}

		for _, media := range decoded.Data.Page.Media {
			banner := media.BannerImage
			poster := firstNonEmpty(media.CoverImage.ExtraLarge, media.CoverImage.Large, media.CoverImage.Medium)
			if banner == "" && poster == "" {
				continue
			}

			candidates := uniqueStrings(media.Title.UserPreferred, media.Title.English, media.Title.Romaji, media.Title.Native)
			conf, _ := bestTitleMatch(seriesTitles, candidates)
			if conf < 0.85 || conf <= bestConf {
				continue
			}

			best = &Artwork{
				PosterURL: poster,
				BannerURL: banner,
				Source:    "anilist",
			}
			bestConf = conf
			if bestConf >= 0.96 {
				return best, nil
			}
		}
	}

	return best, nil
}
