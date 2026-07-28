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
