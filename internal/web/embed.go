package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// SPAHandler serves the embedded Svelte SPA.
// It serves static files from dist/ and falls back to index.html
// for all non-file routes (client-side routing support).
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

		// Try to open the file from the embedded FS
		f, err := sub.Open(cleanPath)
		if err == nil {
			f.Close()
			// File exists, serve it directly
			fileServer.ServeHTTP(w, r)
			return
		}

		// File doesn't exist -- serve index.html for SPA client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
