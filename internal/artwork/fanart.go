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

type fanartResp struct {
	Name     string `json:"name"`
	TvdbID   string `json:"thetvdb_id"`
	Tvposter []struct {
		URL string `json:"url"`
	} `json:"tvposter"`
	Tvbanner []struct {
		URL string `json:"url"`
	} `json:"tvbanner"`
	Showbackground []struct {
		URL string `json:"url"`
	} `json:"showbackground"`
	Hdtvlogo []struct {
		URL string `json:"url"`
	} `json:"hdtvlogo"`
}

func (p *FanartProvider) Fetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	// Fanart.tv needs a TVDB ID. Sonarr series may have it in the
	// series metadata, but our SonarrSeries model doesn't expose it.
	// We'll try searching by name as a fallback via the fanart API.
	// Actually, fanart.tv's API is: GET /tv/{tvdb_id}
	// Without the TVDB ID, we can't use this provider directly.
	// However, Sonarr stores the TVDB ID — let's check if it's available.
	tvdbID := getTVDBID(series)
	if tvdbID == "" {
		// Can't use Fanart.tv without TVDB ID
		return nil, nil
	}

	client := p.httpClient
	url := fmt.Sprintf("https://webservice.fanart.tv/v3.2/tv/%s?api_key=%s", tvdbID, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mizu/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
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

	// Clearart logo (transparent background)
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

// getTVDBID extracts the TVDB ID from Sonarr series metadata.
func getTVDBID(series models.SonarrSeries) string {
	if series.TVDBID != 0 {
		return fmt.Sprintf("%d", series.TVDBID)
	}
	return ""
}
