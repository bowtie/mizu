package artwork

import (
	"context"
	"mizu/internal/models"
)

// Artwork holds remote URLs returned by a provider. The Manager downloads
// these to disk and stores the local paths in the database.
type Artwork struct {
	PosterURL string `json:"poster_url,omitempty"`
	BannerURL string `json:"banner_url,omitempty"`
	LogoURL   string `json:"logo_url,omitempty"`
	Source    string `json:"source"`
}

// Provider fetches artwork URLs from an external API.
type Provider interface {
	// Name is the unique identifier for this provider (e.g. "anilist", "tmdb").
	Name() string

	// Available reports whether the provider is configured (API keys set, etc.).
	Available() bool

	// Fetch searches for the series and returns the best-matching artwork URLs.
	Fetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error)
}

// ProviderInfo describes a provider for the frontend.
type ProviderInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	NeedsKey  bool   `json:"needs_key"`
	Available bool   `json:"available"`
}

// ArtworkOption is a single candidate image a provider can offer. The
// picker shows the thumbnail and, when chosen, downloads the full-size URL.
type ArtworkOption struct {
	URL      string `json:"url"`
	ThumbURL string `json:"thumb_url,omitempty"`
	Lang     string `json:"lang,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Likes    int    `json:"likes,omitempty"`
}

// ArtworkOptions is the full set of candidate images for one series. A
// provider that only has a single best pick per slot simply returns one
// entry per list; multi-image sources (fanart, tmdb) return all of them.
type ArtworkOptions struct {
	Posters []ArtworkOption `json:"posters"`
	Banners []ArtworkOption `json:"banners"`
	Logos   []ArtworkOption `json:"logos"`
}

// CandidatesProvider is an optional extension a Provider can implement to
// expose every candidate image it found instead of only the single best one.
// Sources like fanart.tv and TMDB return many posters/banners/logos; the
// artwork picker lists them so the user can choose which one is stored.
type CandidatesProvider interface {
	Provider
	Candidates(ctx context.Context, series models.SonarrSeries) (*ArtworkOptions, error)
}
