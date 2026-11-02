package web

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// indexETag is a weak ETag for index.html, computed once at startup.
// Since index.html changes when the build changes (asset hashes change),
// this is stable within a single server binary version.
var indexETag string

// CSP is the Content-Security-Policy for the app. Scripts may only come
// from Mizu itself, plus the inline scripts in the built index.html, which
// are allowed by hash (computed below) so no other inline script can run.
// Images may come from anywhere: posters are loaded from Sonarr and
// artwork providers directly. Styles allow inline because the component
// library sets style attributes.
var CSP = buildCSP(nil)

func buildCSP(scriptHashes []string) string {
	scriptSrc := "'self'"
	for _, h := range scriptHashes {
		scriptSrc += " '" + h + "'"
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + scriptSrc,
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: http: https:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"worker-src 'self'",
		"manifest-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

// inlineScript matches a <script> element without a src attribute.
var (
	inlineScript = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	srcAttr      = regexp.MustCompile(`(?i)\ssrc\s*=`)
)

// inlineScriptHashes returns the CSP hash source for every inline script
// in html.
func inlineScriptHashes(html []byte) []string {
	var hashes []string
	for _, m := range inlineScript.FindAllSubmatch(html, -1) {
		if srcAttr.Match(m[1]) {
			continue
		}
		sum := sha256.Sum256(m[2])
		hashes = append(hashes, "sha256-"+base64.StdEncoding.EncodeToString(sum[:]))
	}
	return hashes
}

func init() {
	// Compute a hash of index.html for ETag-based conditional requests.
	// This lets the browser send If-None-Match on repeat visits and get
	// a 304 Not Modified response instead of re-downloading the HTML.
	data, err := distFS.ReadFile("dist/index.html")
	if err == nil {
		h := sha256.Sum256(data)
		indexETag = "W/\"" + hex.EncodeToString(h[:8]) + "\""
		CSP = buildCSP(inlineScriptHashes(data))
	}
}

// SPAHandler serves the embedded React SPA.
// It serves static files from dist/ and falls back to index.html
// for all non-file routes (client-side routing support).
// Hashed assets in /assets/ get immutable cache headers (1 year)
// since their filenames change when content changes. index.html
// gets no-cache with ETag so the browser always validates but only
// re-downloads when the server binary changes.
func SPAHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Strip leading slash for fs.Open
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Try to open the file from the embedded FS. A directory (e.g.
		// /assets) counts as a miss — otherwise http.FileServer would
		// render a listing of embedded file names.
		f, err := sub.Open(cleanPath)
		if err == nil {
			isDir := false
			if st, serr := f.Stat(); serr == nil {
				isDir = st.IsDir()
			}
			f.Close()
			if !isDir {
				// Hashed assets in /assets/ are safe to cache forever —
				// their filenames change when content changes (Vite hash).
				// Fonts are also immutable.
				if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/fonts/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				// Service worker must always be revalidated so the browser
				// picks up updates promptly. Without this, heuristic caching
				// could serve a stale SW for up to 24h.
				if path == "/sw.js" {
					w.Header().Set("Cache-Control", "no-cache")
				}
				// File exists, serve it directly
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// File doesn't exist -- serve index.html for SPA client-side routing.
		// Use no-cache + ETag so the browser always validates but gets a
		// 304 Not Modified when the server binary hasn't changed.
		w.Header().Set("Cache-Control", "no-cache")
		if indexETag != "" {
			w.Header().Set("ETag", indexETag)
			// Check conditional request — if the ETag matches, skip
			// sending the body. The browser still made a request but
			// saves the bandwidth of re-downloading the HTML.
			if match := r.Header.Get("If-None-Match"); match == indexETag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
