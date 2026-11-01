package artwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mizu/internal/models"
)

// FanartProvider fetches artwork from Fanart.tv.
// Requires an API key (free registration at fanart.tv).
// Needs the TVDB ID from the Sonarr series metadata.
type FanartProvider struct {
	apiKey     string
	httpClient *http.Client
}

func NewFanartProvider(apiKey string, timeout time.Duration) *FanartProvider {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &FanartProvider{apiKey: apiKey, httpClient: &http.Client{Timeout: timeout}}
}

func (p *FanartProvider) Name() string    { return "fanart" }
func (p *FanartProvider) Available() bool { return p.apiKey != "" }

// fanartImage is one entry in a fanart.tv section. `Likes` is a JSON string
// in the fanart API (e.g. "5"), so it's parsed separately.
type fanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Lang  string `json:"lang"`
	Likes string `json:"likes"`
}

type fanartResp struct {
	Name           string        `json:"name"`
	TvdbID         string        `json:"thetvdb_id"`
	Tvposter       []fanartImage `json:"tvposter"`
	Tvbanner       []fanartImage `json:"tvbanner"`
	Showbackground []fanartImage `json:"showbackground"`
	Hdtvlogo       []fanartImage `json:"hdtvlogo"`
	Tvthumb        []fanartImage `json:"tvthumb"`
}

func (p *FanartProvider) Fetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	decoded, err := p.fetchResponse(ctx, series)
	if err != nil || decoded == nil {
		return nil, err
	}

	poster := ""
	banner := ""
	logo := ""

	// Prefer showbackground (1920x1080) for hero, then tvbanner
	if len(decoded.Showbackground) > 0 && decoded.Showbackground[0].URL != "" {
		banner = decoded.Showbackground[0].URL
	} else if len(decoded.Tvbanner) > 0 && decoded.Tvbanner[0].URL != "" {
		banner = decoded.Tvbanner[0].URL
	}

	if len(decoded.Tvposter) > 0 && decoded.Tvposter[0].URL != "" {
		poster = decoded.Tvposter[0].URL
	}

	// HDTV logo (transparent background)
	if len(decoded.Hdtvlogo) > 0 && decoded.Hdtvlogo[0].URL != "" {
		logo = decoded.Hdtvlogo[0].URL
	}

	if poster == "" && banner == "" && logo == "" {
		return nil, nil
	}

	return &Artwork{
		PosterURL: poster,
		BannerURL: banner,
		LogoURL:   logo,
		Source:    "fanart",
	}, nil
}

// Candidates returns every poster, banner, and logo fanart.tv has for the
// series so the picker can let the user choose. Poster thumbnails come from
// the tvthumb section (same id); banners/logos reuse the full image as their
// thumbnail since fanart.tv provides no preview for those sections.
func (p *FanartProvider) Candidates(ctx context.Context, series models.SonarrSeries) (*ArtworkOptions, error) {
	decoded, err := p.fetchResponse(ctx, series)
	if err != nil || decoded == nil {
		return nil, err
	}

	thumbs := make(map[string]string, len(decoded.Tvthumb))
	for _, t := range decoded.Tvthumb {
		if t.ID != "" && t.URL != "" {
			thumbs[t.ID] = t.URL
		}
	}

	opts := &ArtworkOptions{}
	for _, img := range decoded.Tvposter {
		if img.URL == "" {
			continue
		}
		opts.Posters = append(opts.Posters, ArtworkOption{
			URL:      img.URL,
			ThumbURL: thumbs[img.ID],
			Lang:     img.Lang,
			Likes:    atoiLikes(img.Likes),
		})
	}
	// Backdrops first (1920x1080 hero images), then banners.
	for _, img := range decoded.Showbackground {
		if img.URL == "" {
			continue
		}
		opts.Banners = append(opts.Banners, ArtworkOption{
			URL:   img.URL,
			Lang:  img.Lang,
			Likes: atoiLikes(img.Likes),
		})
	}
	for _, img := range decoded.Tvbanner {
		if img.URL == "" {
			continue
		}
		opts.Banners = append(opts.Banners, ArtworkOption{
			URL:   img.URL,
			Lang:  img.Lang,
			Likes: atoiLikes(img.Likes),
		})
	}
	for _, img := range decoded.Hdtvlogo {
		if img.URL == "" {
			continue
		}
		opts.Logos = append(opts.Logos, ArtworkOption{
			URL:   img.URL,
			Lang:  img.Lang,
			Likes: atoiLikes(img.Likes),
		})
	}
	return opts, nil
}

// fetchResponse loads and decodes the fanart.tv response for a series,
// returning nil for a missing TVDB ID or a 404 (no artwork).
func (p *FanartProvider) fetchResponse(ctx context.Context, series models.SonarrSeries) (*fanartResp, error) {
	tvdbID := getTVDBID(series)
	if tvdbID == "" {
		// Can't use Fanart.tv without a TVDB ID.
		return nil, nil
	}

	url := fmt.Sprintf("https://webservice.fanart.tv/v3.2/tv/%s?api_key=%s", tvdbID, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mizu/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil // no artwork for this TVDB ID
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fanart.tv returned %d", resp.StatusCode)
	}

	var decoded fanartResp
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return &decoded, nil
}

// atoiLikes converts fanart.tv's string `likes` field to an int, defaulting
// to 0 on malformed values.
func atoiLikes(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// getTVDBID extracts the TVDB ID from Sonarr series metadata.
func getTVDBID(series models.SonarrSeries) string {
	if series.TVDBID != 0 {
		return fmt.Sprintf("%d", series.TVDBID)
	}
	return ""
}
