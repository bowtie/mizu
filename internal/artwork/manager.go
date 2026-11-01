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
	mapMu    sync.Mutex
	mapLocks map[int64]*sync.Mutex

	// intentMu guards intents. A per-mapping intent counter is bumped by
	// newer user intent (a per-show refresh or an artwork source switch) so
	// an older refresh-all write-back for the same mapping skips instead of
	// clobbering the later choice — the per-mapping lock preserves
	// acquisition order, not recency of intent.
	intentMu sync.Mutex
	intents  map[int64]int64
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
		intents:  make(map[int64]int64),
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

// BumpIntent records a newer user intent for a mapping: a per-show refresh
// or an artwork source switch. In-flight fetches that captured the intent
// before the bump skip their write-backs for that mapping (see
// fetchAndStore), so a newer user choice always wins over an older
// refresh-all write-back that lands later.
func (m *Manager) BumpIntent(mappingID int64) {
	m.intentMu.Lock()
	m.intents[mappingID]++
	m.intentMu.Unlock()
}

// captureIntent returns the mapping's current intent counter.
func (m *Manager) captureIntent(mappingID int64) int64 {
	m.intentMu.Lock()
	defer m.intentMu.Unlock()
	return m.intents[mappingID]
}

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

	// Supersede guard (late): a newer per-show refresh or source switch may
	// have landed while this fetch was downloading — skip the DB write-back
	// and remove the just-downloaded files, so the stale fetch leaves no
	// trace that could render over the newer choice (the newer fetch, queued
	// on the per-mapping lock, rewrites both).
	if m.captureIntent(mappingID) != capturedIntent {
		m.deleteArtworkUnlocked(mappingID)
		return &FetchResult{Source: source, Found: false}, nil
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

	// Capture each mapping's intent at the snapshot moment: a per-show
	// refresh or source switch that lands during the run bumps it, and the
	// per-show fetch for that mapping then skips its write-back instead of
	// clobbering the newer choice (see fetchAndStore). Capturing here — not
	// per mapping as the loop reaches it — is what makes the refresh-all the
	// older intent for every mapping it touches.
	snapshot := make(map[int64]int64, len(mappings))
	for _, mapping := range mappings {
		snapshot[mapping.ID] = m.captureIntent(mapping.ID)
	}

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

		fr, err := m.fetchAndStore(ctx, mapping.ID, series, source, snapshot[mapping.ID])
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

// deleteArtworkUnlocked removes all downloaded artwork files for a
// mapping. The caller must hold the per-mapping lock.
func (m *Manager) deleteArtworkUnlocked(mappingID int64) {
	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	_ = os.RemoveAll(dir)
}

// DeleteArtwork removes all downloaded artwork files for a mapping.
// It takes the per-mapping lock so an in-flight fetch for the same mapping
// cannot resurrect files after the deletion: the fetch either completes
// before the delete runs (and the delete then removes its output), or it
// starts after and skips via the mapping-existence check in FetchAndStore.
// Called when a mapping is deleted to prevent orphaned files on disk.
func (m *Manager) DeleteArtwork(mappingID int64) {
	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()
	m.deleteArtworkUnlocked(mappingID)
}

// ClearArtworkForSonarr removes a mapping's local artwork files and clears
// its DB file paths, serialized against in-flight fetches via the per-mapping
// lock. Called when switching a mapping's source to "sonarr" so the frontend
// falls back to Sonarr image URLs instead of stale downloads — without the
// lock, an in-flight fetch completing after the switch would re-populate the
// files and DB paths, silently undoing it.
func (m *Manager) ClearArtworkForSonarr(ctx context.Context, mappingID int64) error {
	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()
	return m.clearArtworkForSonarrUnlocked(ctx, mappingID)
}

// clearArtworkForSonarrUnlocked removes a mapping's local artwork files and
// clears its DB file paths. The caller must hold the per-mapping lock:
// ClearArtworkForSonarr acquires it and delegates here, while fetchAndStore
// calls it directly because it already holds the lock for the same mapping.
func (m *Manager) clearArtworkForSonarrUnlocked(ctx context.Context, mappingID int64) error {
	m.deleteArtworkUnlocked(mappingID)
	return m.db.UpdateArtworkFiles(ctx, mappingID, "", "", "")
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
