package artwork

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // decoders for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"slices"
)

const (
	// MaxCustomImageBytes caps an uploaded image.
	MaxCustomImageBytes = 10 << 20
	// maxCustomImageSide caps an uploaded image's width or height; anything
	// larger is almost certainly not artwork and costs real memory to decode.
	maxCustomImageSide = 10000
)

// Slots an image can occupy.
const (
	SlotPoster = "poster"
	SlotBanner = "banner"
	SlotLogo   = "logo"
)

// ErrInvalidCustomImage marks an upload that is not an acceptable image; its
// message is safe to show to the user.
var ErrInvalidCustomImage = errors.New("invalid image")

// IsSlot reports whether s names an artwork slot.
func IsSlot(s string) bool { return s == SlotPoster || s == SlotBanner || s == SlotLogo }

// customFileName is where a slot's image lives inside a mapping's artwork
// directory. These match the names provider downloads use, so serving and
// the supersede/cleanup code treat custom and fetched images alike.
func customFileName(slot string) string {
	switch slot {
	case SlotPoster:
		return "poster.jpg"
	case SlotBanner:
		return "banner.jpg"
	default:
		return "logo.png"
	}
}

// validateCustomImage checks that data is a JPEG, PNG, GIF or WebP of a
// sane size. The bytes are stored as uploaded (no re-encode), so what
// the user chose is exactly what is shown.
func validateCustomImage(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("%w: the file is empty", ErrInvalidCustomImage)
	}
	if len(data) > MaxCustomImageBytes {
		return fmt.Errorf("%w: images can be at most %d MB", ErrInvalidCustomImage, MaxCustomImageBytes>>20)
	}
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png", "image/gif":
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%w: the file could not be read as an image", ErrInvalidCustomImage)
		}
		if cfg.Width > maxCustomImageSide || cfg.Height > maxCustomImageSide {
			return fmt.Errorf("%w: images can be at most %d pixels wide or tall", ErrInvalidCustomImage, maxCustomImageSide)
		}
	case "image/webp":
		// No decoder is bundled for WebP; the size cap above is the guard.
	default:
		return fmt.Errorf("%w: use a JPG, PNG, GIF or WebP file", ErrInvalidCustomImage)
	}
	return nil
}

// SetCustomArtwork stores a user-uploaded image in one slot. It records
// newer intent so an in-flight refresh can't overwrite the upload, flags the
// slot custom so later refreshes and source switches leave it alone, and
// leaves every other slot as it was.
func (m *Manager) SetCustomArtwork(ctx context.Context, mappingID int64, slot string, data []byte) error {
	if !IsSlot(slot) {
		return fmt.Errorf("%w: unknown slot %q", ErrInvalidCustomImage, slot)
	}
	if err := validateCustomImage(data); err != nil {
		return err
	}

	m.BumpIntent(mappingID)
	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()

	current, err := m.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return fmt.Errorf("load mapping for artwork upload: %w", err)
	}
	if current == nil {
		return errors.New("mapping not found")
	}

	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create artwork dir: %w", err)
	}
	if slot == SlotLogo {
		// A previous SVG logo would otherwise keep winning in LogoPath.
		_ = os.Remove(filepath.Join(dir, "logo.svg"))
	}
	dest := filepath.Join(dir, customFileName(slot))
	// Write then rename so a failed write can't leave a half-written image
	// where the previous one was.
	tmp := dest + ".upload"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write artwork: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("store artwork: %w", err)
	}

	rel := relPath(m.artDir, dest)
	poster, banner, logo := current.ArtworkPosterPath, current.ArtworkBannerPath, current.ArtworkLogoPath
	posterURL, bannerURL, logoURL := current.ArtworkPosterURL, current.ArtworkBannerURL, current.ArtworkLogoURL
	// An upload has no remote origin; clearing the URL also stops the picker
	// from ringing a provider image that is no longer the one in use.
	switch slot {
	case SlotPoster:
		poster, posterURL = rel, ""
	case SlotBanner:
		banner, bannerURL = rel, ""
	case SlotLogo:
		logo, logoURL = rel, ""
	}
	if err := m.db.UpdateArtwork(ctx, mappingID, poster, banner, logo, posterURL, bannerURL, logoURL); err != nil {
		return fmt.Errorf("save artwork paths: %w", err)
	}
	return m.db.SetArtworkCustomSlots(ctx, mappingID, withSlot(current.ArtworkCustomSlots, slot))
}

// RemoveCustomArtwork drops an uploaded image from a slot and stops
// protecting it, so the next refresh can fill the slot again. It does not
// refetch by itself.
func (m *Manager) RemoveCustomArtwork(ctx context.Context, mappingID int64, slot string) error {
	if !IsSlot(slot) {
		return fmt.Errorf("%w: unknown slot %q", ErrInvalidCustomImage, slot)
	}
	m.BumpIntent(mappingID)
	m.lockMapping(mappingID).Lock()
	defer m.lockMapping(mappingID).Unlock()

	current, err := m.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return fmt.Errorf("load mapping for artwork removal: %w", err)
	}
	if current == nil {
		return errors.New("mapping not found")
	}
	if !slices.Contains(current.ArtworkCustomSlots, slot) {
		return nil
	}

	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	_ = os.Remove(filepath.Join(dir, customFileName(slot)))

	poster, banner, logo := current.ArtworkPosterPath, current.ArtworkBannerPath, current.ArtworkLogoPath
	switch slot {
	case SlotPoster:
		poster = ""
	case SlotBanner:
		banner = ""
	case SlotLogo:
		logo = ""
	}
	if err := m.db.UpdateArtwork(ctx, mappingID, poster, banner, logo,
		current.ArtworkPosterURL, current.ArtworkBannerURL, current.ArtworkLogoURL); err != nil {
		return fmt.Errorf("clear artwork path: %w", err)
	}
	return m.db.SetArtworkCustomSlots(ctx, mappingID, withoutSlots(current.ArtworkCustomSlots, slot))
}

func withSlot(slots []string, slot string) []string {
	if slices.Contains(slots, slot) {
		return slots
	}
	return append(slices.Clone(slots), slot)
}

func withoutSlots(slots []string, drop ...string) []string {
	out := make([]string, 0, len(slots))
	for _, s := range slots {
		if !slices.Contains(drop, s) {
			out = append(out, s)
		}
	}
	return out
}

// deleteArtworkExceptUnlocked removes a mapping's artwork files but keeps
// those of the given custom slots. The caller must hold the per-mapping lock.
func (m *Manager) deleteArtworkExceptUnlocked(mappingID int64, keep []string) {
	if len(keep) == 0 {
		m.deleteArtworkUnlocked(mappingID)
		return
	}
	dir := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	protected := make(map[string]bool, len(keep))
	for _, slot := range keep {
		protected[customFileName(slot)] = true
	}
	for _, e := range entries {
		if !protected[e.Name()] {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}
