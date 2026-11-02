package artwork

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"mizu/internal/database"
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

	// hashCache remembers perceptual hashes by image URL (uint64) so a
	// candidate listing only downloads each thumbnail once per process.
	hashCache sync.Map
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
	// Uploaded images are the user's own and survive a switch to Sonarr's.
	current, err := m.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return err
	}
	if current == nil || len(current.ArtworkCustomSlots) == 0 {
		m.deleteArtworkUnlocked(mappingID)
		return m.db.UpdateArtwork(ctx, mappingID, "", "", "", "", "", "")
	}
	custom := current.ArtworkCustomSlots
	m.deleteArtworkExceptUnlocked(mappingID, custom)
	keep := func(slot, path, url string) (string, string) {
		if slices.Contains(custom, slot) {
			return path, url
		}
		return "", ""
	}
	poster, posterURL := keep(SlotPoster, current.ArtworkPosterPath, current.ArtworkPosterURL)
	banner, bannerURL := keep(SlotBanner, current.ArtworkBannerPath, current.ArtworkBannerURL)
	logo, logoURL := keep(SlotLogo, current.ArtworkLogoPath, current.ArtworkLogoURL)
	return m.db.UpdateArtwork(ctx, mappingID, poster, banner, logo, posterURL, bannerURL, logoURL)
}

// FetchResult is returned by FetchAndStore.
type FetchResult struct {
	Source     string `json:"source"`
	Found      bool   `json:"found"`
	PosterPath string `json:"poster_path,omitempty"`
	BannerPath string `json:"banner_path,omitempty"`
	LogoPath   string `json:"logo_path,omitempty"`
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
