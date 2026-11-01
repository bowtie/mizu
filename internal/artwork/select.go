package artwork

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// SelectArtwork downloads specific user-picked image URLs for a mapping and
// persists them, replacing any auto-fetched artwork. It records newer intent
// (like a per-show refresh) so an in-flight refresh-all skips its write-back
// instead of clobbering the pick.
func (m *Manager) SelectArtwork(ctx context.Context, mappingID int64, source, posterURL, bannerURL, logoURL string) (*FetchResult, error) {
	m.BumpIntent(mappingID)
	captured := m.captureIntent(mappingID)

	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()

	// Same existence/override guards as fetchAndStore: a delete or a switch
	// to "sonarr" that landed before this pick must not resurrect files.
	current, err := m.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return nil, fmt.Errorf("load mapping for artwork select: %w", err)
	}
	if current == nil {
		return &FetchResult{Source: source, Found: false}, nil
	}
	if current.ArtworkSource == "sonarr" {
		if err := m.clearArtworkForSonarrUnlocked(ctx, mappingID); err != nil {
			log.Warn().Err(err).Int64("mapping_id", mappingID).Msg("failed to clear artwork file paths")
		}
		return &FetchResult{Source: source, Found: false}, nil
	}

	// An all-empty selection is a no-op (nothing to pick) — never clear the
	// user's existing artwork just because a slot was left empty.
	if posterURL == "" && bannerURL == "" && logoURL == "" {
		return &FetchResult{Source: source, Found: false}, nil
	}

	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create artwork dir: %w", err)
	}

	result := &FetchResult{Source: source, Found: true}
	// Track the files this pick wrote so a superseded pick can remove exactly
	// its own writes without disturbing the untouched slots.
	var downloaded []string

	if posterURL != "" {
		path, err := m.downloadImage(ctx, posterURL, filepath.Join(dir, "poster.jpg"))
		if err == nil {
			result.PosterPath = path
			downloaded = append(downloaded, path)
		} else {
			log.Warn().Err(err).Str("url", posterURL).Int64("mapping_id", mappingID).Msg("failed to download poster")
		}
	}
	if bannerURL != "" {
		path, err := m.downloadImage(ctx, bannerURL, filepath.Join(dir, "banner.jpg"))
		if err == nil {
			result.BannerPath = path
			downloaded = append(downloaded, path)
		} else {
			log.Warn().Err(err).Str("url", bannerURL).Int64("mapping_id", mappingID).Msg("failed to download banner")
		}
	}
	if logoURL != "" {
		// Remove any previous logo first — the new one may have a different
		// extension (png vs svg), and LogoPath() would otherwise keep
		// preferring the stale file.
		for _, name := range []string{"logo.png", "logo.svg"} {
			_ = os.Remove(filepath.Join(dir, name))
		}
		ext := "logo.png"
		if isSVGURL(logoURL) {
			ext = "logo.svg"
		}
		path, err := m.downloadImage(ctx, logoURL, filepath.Join(dir, ext))
		if err == nil {
			result.LogoPath = path
			downloaded = append(downloaded, path)
		} else {
			log.Warn().Err(err).Str("url", logoURL).Int64("mapping_id", mappingID).Msg("failed to download logo")
		}
	}

	// Supersede guard: a newer refresh/switch/pick may have landed while this
	// pick was downloading. Skip the DB write-back and remove only the files
	// this pick wrote so the stale pick leaves no trace.
	if m.captureIntent(mappingID) != captured {
		for _, p := range downloaded {
			_ = os.Remove(p)
		}
		// Remove the dir only if this pick left it empty (untouched slots'
		// files keep it non-empty, so this no-ops for partial picks).
		_ = os.Remove(dir)
		return &FetchResult{Source: source, Found: false}, nil
	}

	// Partial write-back: empty slots keep the mapping's existing paths/URLs.
	relPoster := relPath(m.artDir, result.PosterPath)
	if relPoster == "" {
		relPoster = current.ArtworkPosterPath
	}
	relBanner := relPath(m.artDir, result.BannerPath)
	if relBanner == "" {
		relBanner = current.ArtworkBannerPath
	}
	relLogo := relPath(m.artDir, result.LogoPath)
	if relLogo == "" {
		relLogo = current.ArtworkLogoPath
	}
	// Same preserve-empty rule for the source URLs: a partial pick that
	// leaves a slot untouched keeps the existing URL (needed by the picker
	// to ring the tile that is currently in use).
	if posterURL == "" {
		posterURL = current.ArtworkPosterURL
	}
	if bannerURL == "" {
		bannerURL = current.ArtworkBannerURL
	}
	if logoURL == "" {
		logoURL = current.ArtworkLogoURL
	}
	if err := m.db.UpdateArtwork(ctx, mappingID, relPoster, relBanner, relLogo, posterURL, bannerURL, logoURL); err != nil {
		log.Error().Err(err).Int64("mapping_id", mappingID).Msg("failed to update artwork file paths in DB")
	}

	return result, nil
}
