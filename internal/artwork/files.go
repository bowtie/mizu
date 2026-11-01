package artwork

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// PosterPath returns the on-disk path for a mapping's poster, or "" if none.
func (m *Manager) PosterPath(mappingID int64) string {
	p := filepath.Join(m.artDir, fmt.Sprintf("%d", mappingID), "poster.jpg")
	if fileExists(p) {
		return p
	}
	return ""
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
	// Sandboxed serving: SVGs (and any mis-typed file) loaded as a
	// top-level document must not run scripts or ride this origin's
	// identity — "sandbox" denies both, nosniff blocks MIME confusion.
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
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
