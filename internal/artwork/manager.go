package artwork

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mizu/internal/database"
	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

// Manager coordinates artwork providers, downloads images to disk, and
// serves them locally so the app works offline.
type Manager struct {
	mu         sync.RWMutex
	providers  map[string]Provider
	order      []string // provider names in priority order
	artDir     string
	httpClient *http.Client
	db         *database.DB

	// Per-mapping locks prevent concurrent FetchAndStore calls for the
	// same mapping from corrupting each other's file writes.
	mapMu   sync.Mutex
	mapLocks map[int64]*sync.Mutex
}

// NewManager creates a Manager that stores images under artDir.
func NewManager(artDir string, db *database.DB) *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		artDir:    artDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		db:       db,
		mapLocks: make(map[int64]*sync.Mutex),
	}
}

// Register adds a provider to the manager.
func (m *Manager) Register(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
	m.order = append(m.order, p.Name())
}

// UpdateProviders replaces all providers. Called when settings (e.g. API keys)
// change at runtime so the new keys take effect without restarting the server.
func (m *Manager) UpdateProviders(providers []Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = make(map[string]Provider, len(providers))
	m.order = make([]string, 0, len(providers))
	for _, p := range providers {
		m.providers[p.Name()] = p
		m.order = append(m.order, p.Name())
	}
}

// Providers returns info about all registered providers for the frontend.
func (m *Manager) Providers() []ProviderInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ProviderInfo, 0, len(m.order))
	for _, name := range m.order {
		p := m.providers[name]
		result = append(result, ProviderInfo{
			ID:        p.Name(),
			Label:     providerLabel(p.Name()),
			NeedsKey:  providerNeedsKey(p.Name()),
			Available: p.Available(),
		})
	}
	return result
}

// lockMapping returns a per-mapping mutex, creating it if needed.
// This prevents concurrent FetchAndStore calls for the same mapping
// from interleaving file operations (delete + create + download).
func (m *Manager) lockMapping(mappingID int64) *sync.Mutex {
	m.mapMu.Lock()
	defer m.mapMu.Unlock()
	if m.mapLocks[mappingID] == nil {
		m.mapLocks[mappingID] = &sync.Mutex{}
	}
	return m.mapLocks[mappingID]
}

// ResolveSource returns the effective source for a mapping: the per-show
// override if set, otherwise the global default.
func (m *Manager) ResolveSource(override, defaultSource string) string {
	if override != "" {
		return override
	}
	return defaultSource
}

// FetchAndStore fetches artwork from the specified source, downloads the
// images to disk, and updates the database with local paths. A "sonarr" or
// empty source skips provider fetch — the frontend will fall back to Sonarr
// image URLs.
func (m *Manager) FetchAndStore(ctx context.Context, mappingID int64, series models.SonarrSeries, source string) (*FetchResult, error) {
	// Lock per-mapping to prevent concurrent file operations from
	// corrupting each other (e.g. background artwork fetch overlapping
	// with a manual refresh for the same show).
	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()

	art, err := m.fetchFromSource(ctx, series, source)
	if err != nil {
		return nil, err
	}
	if art == nil {
		return &FetchResult{Source: source, Found: false}, nil
	}

	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create artwork dir: %w", err)
	}

	// Clear old artwork files so stale images from a previous provider
	// don't linger when the new provider doesn't return all three types.
	m.DeleteArtwork(mappingID)
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

	// Store relative paths in DB (relative to artDir).
	// Only update the file paths, not the source override — the override
	// is managed separately via UpdateArtworkSource.
	relPoster := relPath(m.artDir, result.PosterPath)
	relBanner := relPath(m.artDir, result.BannerPath)
	relLogo := relPath(m.artDir, result.LogoPath)
	if err := m.db.UpdateArtworkFiles(ctx, mappingID, relPoster, relBanner, relLogo); err != nil {
		log.Error().Err(err).Int64("mapping_id", mappingID).Msg("failed to update artwork file paths in DB")
	}

	return result, nil
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

// PosterPath returns the on-disk path for a mapping's poster, or "" if none.
func (m *Manager) PosterPath(mappingID int64) string {
	p := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID), "poster.jpg")
	if fileExists(p) {
		return p
	}
	return ""
}

// RefreshResult is the per-show outcome of a RefreshAll operation.
type RefreshResult struct {
	MappingID int64  `json:"mapping_id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Found     bool   `json:"found"`
	Error     string `json:"error,omitempty"`
}

// RefreshAllSummary is the aggregate result returned by RefreshAll.
type RefreshAllSummary struct {
	Total     int             `json:"total"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	NotFound  int             `json:"not_found"`
	Results   []RefreshResult `json:"results"`
}

// RefreshAll iterates over every mapping and re-downloads artwork from the
// resolved source (per-show override > global default). It fetches series
// metadata from Sonarr when available. The seriesFetcher callback lets the
// caller decide how to obtain Sonarr series data (synced or live).
func (m *Manager) RefreshAll(ctx context.Context, mappings []models.ShowMapping, defaultSource string, seriesFetcher func(ctx context.Context, sonarrSeriesID int) (*models.SonarrSeries, error)) RefreshAllSummary {
	summary := RefreshAllSummary{Total: len(mappings), Results: make([]RefreshResult, 0, len(mappings))}

	for _, mapping := range mappings {
		select {
		case <-ctx.Done():
			return summary
		default:
		}

		result := RefreshResult{MappingID: mapping.ID, Title: mapping.SonarrTitle}
		source := m.ResolveSource(mapping.ArtworkSource, defaultSource)
		result.Source = source

		series := models.SonarrSeries{
			ID:    mapping.SonarrSeriesID,
			Title: mapping.SonarrTitle,
		}
		if seriesFetcher != nil {
			if fetched, err := seriesFetcher(ctx, mapping.SonarrSeriesID); err == nil && fetched != nil {
				series = *fetched
			} else if err != nil {
				log.Warn().Err(err).Int("sonarr_series_id", mapping.SonarrSeriesID).Msg("failed to fetch Sonarr series for artwork refresh")
			}
		}
		// AFL slug is a clean, normalized title — add it as an alternate title
		// so providers can use it for matching when the Sonarr title is corrupted.
		if mapping.AFLSlug != "" {
			series.AlternateTitles = append(series.AlternateTitles, models.SonarrAltTitle{Title: mapping.AFLSlug})
		}

		fr, err := m.FetchAndStore(ctx, mapping.ID, series, source)
		if err != nil {
			result.Error = err.Error()
			summary.Failed++
		} else if fr != nil && fr.Found {
			result.Found = true
			summary.Succeeded++
		} else {
			summary.NotFound++
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}

// DeleteArtwork removes all downloaded artwork files for a mapping.
// Called when a mapping is deleted to prevent orphaned files on disk.
func (m *Manager) DeleteArtwork(mappingID int64) {
	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	_ = os.RemoveAll(dir)
}

// BannerPath returns the on-disk path for a mapping's banner, or "" if none.
func (m *Manager) BannerPath(mappingID int64) string {
	p := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID), "banner.jpg")
	if fileExists(p) {
		return p
	}
	return ""
}

// LogoPath returns the on-disk path to the stored logo (PNG or SVG), or "".
func (m *Manager) LogoPath(mappingID int64) string {
	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	for _, name := range []string{"logo.png", "logo.svg"} {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// ServeImage streams a file from disk to the response with the correct
// content type, Last-Modified header, and range-request support.
func ServeImage(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// ETag from mod time so the browser revalidates after a refresh
	// replaces the file on disk with a new provider's image.
	etag := fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size())
	w.Header().Set("ETag", etag)
	// Long cache with ETag revalidation — artwork only changes on explicit
	// refresh (which busts the browser cache via ?v=N query param) or
	// source switch. The ETag from mod-time ensures correctness if the
	// file is replaced without the frontend knowing.
	w.Header().Set("Cache-Control", "max-age=86400, must-revalidate")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentTypeFor(path))
	// http.ServeContent handles Last-Modified, Range requests, and
	// streams the file without reading it all into memory.
	http.ServeContent(w, r, path, info.ModTime(), f)
}

// FetchResult is returned by FetchAndStore.
type FetchResult struct {
	Source     string `json:"source"`
	Found      bool   `json:"found"`
	PosterPath string `json:"poster_path,omitempty"`
	BannerPath string `json:"banner_path,omitempty"`
	LogoPath   string `json:"logo_path,omitempty"`
}

// --- helpers ---

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isSVGURL(u string) bool {
	return strings.HasSuffix(strings.ToLower(u), ".svg") ||
		strings.Contains(strings.ToLower(u), ".svg?")
}

func relPath(base, full string) string {
	if full == "" {
		return ""
	}
	rel, err := filepath.Rel(base, full)
	if err != nil {
		return full
	}
	return rel
}

func contentTypeFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func providerLabel(id string) string {
	switch id {
	case "anilist":
		return "AniList"
	case "tmdb":
		return "TMDB"
	case "fanart":
		return "Fanart.tv"
	case "kitsu":
		return "Kitsu"
	default:
		return id
	}
}

func providerNeedsKey(id string) bool {
	switch id {
	case "tmdb", "fanart":
		return true
	default:
		return false
	}
}
