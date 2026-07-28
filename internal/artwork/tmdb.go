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

// TMDBProvider fetches artwork from The Movie Database.
// Requires an API key (free registration at themoviedb.org).
type TMDBProvider struct {
	apiKey     string
	httpClient *http.Client
}

func NewTMDBProvider(apiKey string, timeout time.Duration) *TMDBProvider {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &TMDBProvider{apiKey: apiKey, httpClient: &http.Client{Timeout: timeout}}
}

func (p *TMDBProvider) Name() string    { return "tmdb" }
func (p *TMDBProvider) Available() bool { return p.apiKey != "" }

const tmdbBase = "https://api.themoviedb.org/3"
const tmdbImageBase = "https://image.tmdb.org/t/p/original"

type tmdbSearchResp struct {
	Results []struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Overview     string `json:"overview"`
		PosterPath   string `json:"poster_path"`
		BackdropPath string `json:"backdrop_path"`
	} `json:"results"`
	TotalResults int `json:"total_results"`
}

// tmdbImagesResp is the response from /tv/{id}/images — includes all
// backdrops, posters, and logos with vote averages so we can pick the best one.
type tmdbImagesResp struct {
	Backdrops []struct {
		FilePath  string  `json:"file_path"`
		VoteAvg   float64 `json:"vote_average"`
		VoteCount int     `json:"vote_count"`
		Language  string  `json:"iso_639_1"`
		Width     int     `json:"width"`
		Height    int     `json:"height"`
	} `json:"backdrops"`
	Posters []struct {
		FilePath  string  `json:"file_path"`
		VoteAvg   float64 `json:"vote_average"`
		VoteCount int     `json:"vote_count"`
		Language  string  `json:"iso_639_1"`
		Width     int     `json:"width"`
		Height    int     `json:"height"`
	} `json:"posters"`
	Logos []struct {
		FilePath  string  `json:"file_path"`
		VoteAvg   float64 `json:"vote_average"`
		VoteCount int     `json:"vote_count"`
		Language  string  `json:"iso_639_1"`
		Width     int     `json:"width"`
		Height    int     `json:"height"`
	} `json:"logos"`
}

func (p *TMDBProvider) Fetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	client := p.httpClient
	titles := searchTitles(series)

	var bestID int
	var bestConf float64

	// Phase 1: search to find the best-matching TMDB TV ID.
	for _, title := range titles {
		u := fmt.Sprintf("%s/search/tv?query=%s&language=en-US&page=1&include_adult=false",
			tmdbBase, url.QueryEscape(title))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var decoded tmdbSearchResp
		err = json.NewDecoder(resp.Body).Decode(&decoded)
		resp.Body.Close()
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || decoded.TotalResults == 0 {
			continue
		}

		for _, r := range decoded.Results {
			candidates := uniqueStrings(r.Name, r.OriginalName)
			conf, _ := bestTitleMatch(titles, candidates)
			if conf < 0.80 || conf <= bestConf {
				continue
			}
			bestID = r.ID
			bestConf = conf
			if bestConf >= 0.96 {
				break
			}
		}
		if bestConf >= 0.96 {
			break
		}
	}

	if bestID == 0 {
		return nil, nil
	}

	// Phase 2: fetch all images for the matched show and pick the best
	// backdrop/poster/logo by vote average. Falls back to search result's
	// primary images if the images endpoint fails.
	poster, banner, logo := p.fetchBestImages(ctx, bestID)
	if poster == "" && banner == "" {
		// Fallback: re-search to get the primary poster/backdrop.
		for _, title := range titles {
			u := fmt.Sprintf("%s/search/tv?query=%s&language=en-US&page=1&include_adult=false",
				tmdbBase, url.QueryEscape(title))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			var decoded tmdbSearchResp
			err = json.NewDecoder(resp.Body).Decode(&decoded)
			resp.Body.Close()
			if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || decoded.TotalResults == 0 {
				continue
			}
			for _, r := range decoded.Results {
				if r.ID == bestID {
					if r.PosterPath != "" {
						poster = tmdbImageBase + r.PosterPath
					}
					if r.BackdropPath != "" {
						banner = tmdbImageBase + r.BackdropPath
					}
					break
				}
			}
			if poster != "" || banner != "" {
				break
			}
		}
	}

	if poster == "" && banner == "" {
		return nil, nil
	}

	return &Artwork{
		PosterURL: poster,
		BannerURL: banner,
		LogoURL:   logo,
		Source:    "tmdb",
	}, nil
}

// fetchBestImages calls /tv/{id}/images and picks the highest-voted
// backdrop, poster, and logo. Prefers English ("en") language posters/logos
// but falls back to null-language (language-agnostic) ones.
func (p *TMDBProvider) fetchBestImages(ctx context.Context, tmdbID int) (poster, banner, logo string) {
	u := fmt.Sprintf("%s/tv/%d/images?include_image_language=en,null", tmdbBase, tmdbID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", ""
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", ""
	}

	var decoded tmdbImagesResp
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", "", ""
	}

	// Pick the best backdrop by vote average (tie-break: vote count).
	var bestBackdrop struct {
		FilePath  string
		VoteAvg   float64
		VoteCount int
	}
	for _, b := range decoded.Backdrops {
		if b.Width < 1280 {
			continue // skip low-res backdrops
		}
		if b.VoteAvg > bestBackdrop.VoteAvg ||
			(b.VoteAvg == bestBackdrop.VoteAvg && b.VoteCount > bestBackdrop.VoteCount) {
			bestBackdrop.FilePath = b.FilePath
			bestBackdrop.VoteAvg = b.VoteAvg
			bestBackdrop.VoteCount = b.VoteCount
		}
	}
	if bestBackdrop.FilePath != "" {
		banner = tmdbImageBase + bestBackdrop.FilePath
	}

	// Pick the best poster — prefer English, then null-language.
	var bestPoster struct {
		FilePath  string
		VoteAvg   float64
		VoteCount int
	}
	for _, img := range decoded.Posters {
		if img.VoteAvg > bestPoster.VoteAvg ||
			(img.VoteAvg == bestPoster.VoteAvg && img.VoteCount > bestPoster.VoteCount) {
			bestPoster.FilePath = img.FilePath
			bestPoster.VoteAvg = img.VoteAvg
			bestPoster.VoteCount = img.VoteCount
		}
	}
	if bestPoster.FilePath != "" {
		poster = tmdbImageBase + bestPoster.FilePath
	}

	// Pick the best logo — prefer English, then null-language.
	// Prefer SVG (vector) over PNG when vote averages are close.
	var bestLogo struct {
		FilePath  string
		VoteAvg   float64
		VoteCount int
	}
	for _, l := range decoded.Logos {
		if l.VoteAvg > bestLogo.VoteAvg ||
			(l.VoteAvg == bestLogo.VoteAvg && l.VoteCount > bestLogo.VoteCount) {
			bestLogo.FilePath = l.FilePath
			bestLogo.VoteAvg = l.VoteAvg
			bestLogo.VoteCount = l.VoteCount
		}
	}
	if bestLogo.FilePath != "" {
		logo = tmdbImageBase + bestLogo.FilePath
	}

	return poster, banner, logo
}
