package artwork

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

// FetchAndStore fetches artwork from the specified source, downloads the
// images to disk, and updates the database with local paths. A "sonarr" or
// empty source skips provider fetch — the frontend will fall back to Sonarr
// image URLs.
//
// The captured intent is the current one: a per-show fetch records its own
// newer intent (via BumpIntent) before this call, so its write is never
// skipped — only a newer concurrent intent supersedes it.
func (m *Manager) FetchAndStore(ctx context.Context, mappingID int64, series models.SonarrSeries, source string) (*FetchResult, error) {
	return m.fetchAndStore(ctx, mappingID, series, source, m.captureIntent(mappingID))
}

// fetchAndStore is the shared implementation. capturedIntent is the mapping's
// intent when this fetch's work was decided: the current intent for a
// per-show fetch, or the intent at refresh-all start (RefreshAll snapshots it
// for every mapping). If a newer intent lands before the write-back, the
// stale fetch skips writing so it can't clobber the newer artwork.
func (m *Manager) fetchAndStore(ctx context.Context, mappingID int64, series models.SonarrSeries, source string, capturedIntent int64) (*FetchResult, error) {
	// Lock per-mapping to prevent concurrent file operations from
	// corrupting each other (e.g. background artwork fetch overlapping
	// with a manual refresh for the same show).
	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()

	// Supersede guard: a delete or a source switch to "sonarr" may have
	// landed after this fetch was scheduled (the on-create goroutine does a
	// GetSeries call before fetching, and refresh-all iterates a snapshot).
	// Writing now would resurrect files for a mapping that no longer exists,
	// or silently undo a switch back to Sonarr image URLs. The check runs
	// under the per-mapping lock, so it is serialized with the
	// delete/source-switch cleanup that removes the row.
	current, err := m.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return nil, fmt.Errorf("load mapping for artwork fetch: %w", err)
	}
	if current == nil {
		return &FetchResult{Source: source, Found: false}, nil
	}
	if current.ArtworkSource == "sonarr" {
		// The mapping's override is "sonarr": local artwork must not render
		// over the Sonarr image URLs. Clear stale files and DB paths left by
		// an earlier provider fetch. This runs regardless of intent — the
		// mapping's current state wants Sonarr images, so any local downloads
		// are stale by definition (a mid-flight switch already cleared them
		// via ClearArtworkForSonarr, making this a harmless re-clear).
		if err := m.clearArtworkForSonarrUnlocked(ctx, mappingID); err != nil {
			log.Warn().Err(err).Int64("mapping_id", mappingID).Msg("failed to clear artwork file paths")
		}
		return &FetchResult{Source: source, Found: false}, nil
	}

	// Supersede guard (early): a newer per-show refresh or source switch may
	// have landed after this fetch's intent was captured (e.g. a refresh-all
	// reaching a mapping the user refreshed mid-run) — skip the fetch
	// entirely; the newer operation writes its own artwork.
	if m.captureIntent(mappingID) != capturedIntent {
		return &FetchResult{Source: source, Found: false}, nil
	}

	// A fetch that resolves to "sonarr" (or empty) never downloads — the
	// frontend falls back to Sonarr image URLs. Clear any stale local files
	// and DB paths so the sonarr default actually takes effect instead of
	// older downloads rendering over it. The intent guard above ensures a
	// superseded older fetch can't wipe a newer operation's artwork.
	if source == "" || source == "sonarr" {
		if err := m.clearArtworkForSonarrUnlocked(ctx, mappingID); err != nil {
			log.Warn().Err(err).Int64("mapping_id", mappingID).Msg("failed to clear artwork file paths")
		}
		return &FetchResult{Source: source, Found: false}, nil
	}

	art, err := m.fetchFromSource(ctx, series, source)
	if err != nil {
		return nil, err
	}
	if art == nil {
		return &FetchResult{Source: source, Found: false}, nil
	}

	return m.storeArtworkLocked(ctx, mappingID, art, source, capturedIntent)
}

// storeArtworkLocked downloads the URLs in `art` into the mapping's artwork
// directory and writes the local file paths to the DB. The caller must hold
// the per-mapping lock and pass the intent captured when the fetch/selection
// was decided; if a newer intent landed while downloading, the just-written
// files are removed and no DB write happens.
func (m *Manager) storeArtworkLocked(ctx context.Context, mappingID int64, art *Artwork, source string, capturedIntent int64) (*FetchResult, error) {
	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create artwork dir: %w", err)
	}

	// Clear old artwork files so stale images from a previous provider
	// don't linger when the new provider doesn't return all three types.
	m.deleteArtworkUnlocked(mappingID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("recreate artwork dir: %w", err)
	}

	result := &FetchResult{Source: art.Source, Found: true}

	if art.PosterURL != "" {
		path, err := m.downloadImage(ctx, art.PosterURL, filepath.Join(dir, "poster.jpg"))
		if err == nil {
			result.PosterPath = path
		} else {
			log.Warn().Err(err).Str("url", art.PosterURL).Int64("mapping_id", mappingID).Msg("failed to download poster")
		}
	}
	if art.BannerURL != "" {
		path, err := m.downloadImage(ctx, art.BannerURL, filepath.Join(dir, "banner.jpg"))
		if err == nil {
			result.BannerPath = path
		} else {
			log.Warn().Err(err).Str("url", art.BannerURL).Int64("mapping_id", mappingID).Msg("failed to download banner")
		}
	}
	if art.LogoURL != "" {
		ext := "logo.png"
		if isSVGURL(art.LogoURL) {
			ext = "logo.svg"
		}
		path, err := m.downloadImage(ctx, art.LogoURL, filepath.Join(dir, ext))
		if err == nil {
			result.LogoPath = path
		} else {
			log.Warn().Err(err).Str("url", art.LogoURL).Int64("mapping_id", mappingID).Msg("failed to download logo")
		}
	}

	// Supersede guard (late): a newer per-show refresh, source switch, or
	// explicit pick may have landed while these images were downloading —
	// skip the DB write-back and remove the just-downloaded files, so the
	// stale fetch leaves no trace that could render over the newer choice
	// (the newer operation, queued on the per-mapping lock, rewrites both).
	if m.captureIntent(mappingID) != capturedIntent {
		m.deleteArtworkUnlocked(mappingID)
		return &FetchResult{Source: source, Found: false}, nil
	}

	// Store relative paths and their source URLs in DB (relative to artDir).
	// Only update the file paths/URLs, not the source override — the override
	// is managed separately via UpdateArtworkSource.
	relPoster := relPath(m.artDir, result.PosterPath)
	relBanner := relPath(m.artDir, result.BannerPath)
	relLogo := relPath(m.artDir, result.LogoPath)
	if err := m.db.UpdateArtwork(ctx, mappingID, relPoster, relBanner, relLogo, art.PosterURL, art.BannerURL, art.LogoURL); err != nil {
		log.Error().Err(err).Int64("mapping_id", mappingID).Msg("failed to update artwork file paths in DB")
	}

	return result, nil
}

// ListCandidates returns every candidate image a provider offers for a
// series. Sources that don't implement CandidatesProvider (sonarr, anilist,
// kitsu) return an empty set — the caller can then hide the picker.
func (m *Manager) ListCandidates(ctx context.Context, series models.SonarrSeries, source string) (*ArtworkOptions, error) {
	if source == "" || source == "sonarr" || source == "auto" {
		return &ArtworkOptions{}, nil
	}

	m.mu.RLock()
	p, ok := m.providers[source]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown artwork source: %s", source)
	}

	cp, ok := p.(CandidatesProvider)
	if !ok {
		// The source has a single best pick and nothing to browse.
		return &ArtworkOptions{}, nil
	}
	if !p.Available() {
		return nil, fmt.Errorf("provider %s is not configured (missing API key?)", source)
	}
	return cp.Candidates(ctx, series)
}

// fetchFromSource tries the specified source. "auto" tries each provider in
// priority order. "sonarr" (or empty) means no provider fetch — the caller
// should fall back to Sonarr image URLs.
func (m *Manager) fetchFromSource(ctx context.Context, series models.SonarrSeries, source string) (*Artwork, error) {
	if source == "" || source == "sonarr" {
		return nil, nil
	}
	if source == "auto" {
		return m.autoFetch(ctx, series)
	}

	m.mu.RLock()
	p, ok := m.providers[source]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown artwork source: %s", source)
	}
	if !p.Available() {
		return nil, fmt.Errorf("provider %s is not configured (missing API key?)", source)
	}
	return p.Fetch(ctx, series)
}

// autoFetch tries each provider in priority order.
func (m *Manager) autoFetch(ctx context.Context, series models.SonarrSeries) (*Artwork, error) {
	m.mu.RLock()
	order := make([]string, len(m.order))
	copy(order, m.order)
	providers := make(map[string]Provider, len(order))
	for _, name := range order {
		providers[name] = m.providers[name]
	}
	m.mu.RUnlock()

	for _, name := range order {
		p := providers[name]
		if !p.Available() {
			continue
		}
		art, err := p.Fetch(ctx, series)
		if err != nil {
			log.Warn().Err(err).Str("provider", name).Str("series", series.Title).Msg("artwork provider error")
			continue
		}
		if art == nil {
			continue
		}
		if art.PosterURL != "" || art.BannerURL != "" || art.LogoURL != "" {
			return art, nil
		}
	}
	return nil, nil
}

// downloadImage fetches a remote image and saves it to destPath.
func (m *Manager) downloadImage(ctx context.Context, url, destPath string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mizu/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	// Only store actual images: the URL can be user-supplied (artwork
	// picker), and this origin serves whatever lands in the artwork dir.
	// image/svg+xml is allowed here but served sandboxed (see ServeImage)
	// because SVGs can carry scripts.
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(strings.ToLower(ct), "image/") {
		return "", fmt.Errorf("download %s: unexpected content type %q", url, ct)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(out, io.LimitReader(resp.Body, 50<<20)); err != nil {
		out.Close()
		os.Remove(destPath)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(destPath)
		return "", err
	}
	return destPath, nil
}
