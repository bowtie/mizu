# =============================================================================
# AnimeFillerGuard - Windows PowerShell Project Setup Script
# Run with: .\animefillerguard-setup.ps1
# Requires PowerShell 5.1+ (Windows 10/11 built-in)
# =============================================================================

$ErrorActionPreference = "Stop"

Write-Host "==> Creating AnimeFillerGuard project structure..." -ForegroundColor Cyan

New-Item -ItemType Directory -Force -Path "animefillerguard" | Out-Null
Set-Location "animefillerguard"

# Create directories
New-Item -ItemType Directory -Force -Path "cmd" | Out-Null
New-Item -ItemType Directory -Force -Path "cmd/server" | Out-Null
New-Item -ItemType Directory -Force -Path "internal" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/config" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/database" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/matcher" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/models" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/notifier" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/scraper" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/sonarr" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/sync" | Out-Null
New-Item -ItemType Directory -Force -Path "internal/web" | Out-Null
New-Item -ItemType Directory -Force -Path "web" | Out-Null
New-Item -ItemType Directory -Force -Path "web/src" | Out-Null
New-Item -ItemType Directory -Force -Path "web/src/components" | Out-Null
New-Item -ItemType Directory -Force -Path "web/src/lib" | Out-Null
New-Item -ItemType Directory -Force -Path "web/src/pages" | Out-Null

Write-Host "==> Writing project files..." -ForegroundColor Cyan

# --- .gitignore ---
@'
animefillerguard
animefillerguard.exe
*.exe
vendor/
web/node_modules/
internal/web/dist/
config.yml
*.db
.DS_Store
Thumbs.db

'@ | Set-Content -Path ".gitignore" -Encoding UTF8

# --- Dockerfile ---
@'
# =============================================================================
# AnimeFillerGuard - Multi-stage Docker Build
# =============================================================================

# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /build

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with ldflags for version info, static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
        -X main.version=${VERSION} \
        -X main.commit=${COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -o /animefillerguard \
    ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.19

# Install ca-certificates for HTTPS requests + tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 afg && \
    adduser -u 1000 -G afg -s /bin/sh -D afg

# Create config directory
RUN mkdir -p /config && chown afg:afg /config

# Copy binary from builder
COPY --from=builder /animefillerguard /usr/local/bin/animefillerguard

# Environment defaults
ENV SONARR_URL=http://localhost:8989 \
    SONARR_API_KEY= \
    SCAN_INTERVAL=6h \
    LISTEN_ADDR=:8978 \
    DB_PATH=/config/animefillerguard.db \
    LOG_LEVEL=info \
    TZ=UTC

# Expose port
EXPOSE 8978

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8978/api/health || exit 1

# Persistent storage
VOLUME /config

# Run as non-root
USER afg

ENTRYPOINT ["animefillerguard"]
'@ | Set-Content -Path "Dockerfile" -Encoding UTF8

# --- README.md ---
@'
# AnimeFillerGuard

Automatically manage filler episodes in Sonarr using data from [AnimeFillerList](https://www.animefillerlist.com/). Unmonitor, skip, or tag filler and mixed-canon episodes so you only download the episodes that matter.

## Features

- **Automatic filler detection** -- Scrapes AnimeFillerList and cross-references with your Sonarr library
- **Fuzzy matching** -- Jaro-Winkler algorithm matches Sonarr series to AFL entries, even with title variations
- **Configurable actions** -- Per-show rules: unmonitor, keep (monitor), or skip+delete filler episodes
- **Mixed canon support** -- Separate rules for mixed canon/filler episodes
- **Sonarr tagging** -- Optionally tag series that contain filler episodes
- **Push notifications** -- Get notified on sync results via [ntfy.sh](https://ntfy.sh) (self-hosted or public)
- **Web dashboard** -- Dark-themed UI for managing mappings, viewing episodes, and configuring settings
- **Scheduled sync** -- Automatic periodic sync with configurable intervals
- **Docker-ready** -- Single container with embedded SQLite database

## Quick Start

### Docker Compose (Recommended)

```yaml
services:
  animefillerguard:
    image: animefillerguard:latest
    container_name: animefillerguard
    ports:
      - "8978:8978"
    environment:
      - SONARR_URL=http://sonarr:8989
      - SONARR_API_KEY=your-sonarr-api-key
    volumes:
      - ./config:/config
    restart: unless-stopped
```

```bash
docker compose up -d
```

Open `http://localhost:8978` to access the dashboard.

### Docker Run

```bash
docker run -d \
  --name animefillerguard \
  -p 8978:8978 \
  -e SONARR_URL=http://sonarr:8989 \
  -e SONARR_API_KEY=your-sonarr-api-key \
  -v ./config:/config \
  animefillerguard:latest
```

### Build from Source

```bash
# Build frontend
cd web && npm install && npm run build && cd ..

# Build Go binary
go build -o animefillerguard ./cmd/server
```

## Configuration

All settings can be configured via environment variables, a YAML config file, or the web UI. Environment variables take precedence over the config file.

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SONARR_URL` | `http://localhost:8989` | Sonarr instance URL |
| `SONARR_API_KEY` | *(required)* | Sonarr API key (Settings > General > Security) |
| `SCAN_INTERVAL` | `6h` | How often to sync (Go duration or minutes) |
| `LISTEN_ADDR` | `:8978` | HTTP server listen address |
| `BASE_URL` | `/` | Base URL path (for reverse proxies) |
| `DB_PATH` | `/config/animefillerguard.db` | SQLite database path |
| `CACHE_TTL` | `24h` | How long to cache scraped AFL data |
| `DEFAULT_FILLER_ACTION` | `unmonitor` | Default action for filler episodes |
| `DEFAULT_MIXED_CANON_ACTION` | `monitor` | Default action for mixed canon episodes |
| `ENABLE_TAGGING` | `false` | Add a tag to series with filler in Sonarr |
| `FILLER_TAG_NAME` | `filler` | Tag name to use in Sonarr |
| `NTFY_ENABLED` | `false` | Enable ntfy push notifications |
| `NTFY_SERVER_URL` | `https://ntfy.sh` | ntfy server URL |
| `NTFY_TOPIC` | *(empty)* | ntfy topic name |
| `NTFY_TOKEN` | *(empty)* | ntfy access token (optional) |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

### YAML Config File

```yaml
sonarr_url: http://localhost:8989
sonarr_api_key: your-api-key

scan_interval: 6h
listen_addr: ":8978"
db_path: /config/animefillerguard.db
cache_ttl: 24h

default_filler_action: unmonitor    # unmonitor | monitor | skip
default_mixed_canon_action: monitor # unmonitor | monitor | skip

enable_tagging: false
filler_tag_name: filler

ntfy_enabled: false
ntfy_server_url: https://ntfy.sh
ntfy_topic: animefillerguard
ntfy_token: ""

log_level: info
```

Pass the config file with `--config /path/to/config.yml`.

### Filler Actions

| Action | Behavior |
|---|---|
| `unmonitor` | Marks filler episodes as unmonitored in Sonarr (won't be downloaded) |
| `monitor` | Keeps episodes monitored (no change) |
| `skip` | Unmonitors and deletes existing files for filler episodes |

## Architecture

```
cmd/server/
  main.go              # Entry point, wires up all components
  router.go            # HTTP API routes and handlers

internal/
  config/config.go     # YAML + env var configuration
  database/database.go # SQLite with migrations, full CRUD
  matcher/matcher.go   # Jaro-Winkler fuzzy title matching
  models/models.go     # Shared types (Sonarr, AFL, mappings)
  notifier/ntfy.go     # ntfy.sh push notifications
  scraper/scraper.go   # AnimeFillerList HTML parser
  sonarr/sonarr.go     # Sonarr API v3 client
  sync/sync.go         # Orchestrates scraper + matcher + Sonarr
  web/embed.go         # Embeds Svelte SPA into Go binary

web/                   # Svelte SPA frontend
  src/
    components/        # Navbar, Modal, Toast, Badge, Spinner
    pages/             # Dashboard, Shows, ShowDetail, Settings, Logs
    lib/               # API client, stores
```

## Notifications (ntfy)

AnimeFillerGuard can send push notifications after each sync via [ntfy.sh](https://ntfy.sh).

### Setup

1. Install the ntfy app on your phone ([Android](https://play.google.com/store/apps/details?id=io.heckel.ntfy), [iOS](https://apps.apple.com/app/ntfy/id1625396347))
2. Subscribe to your chosen topic (e.g., `animefillerguard`)
3. In Settings > Notifications, enable ntfy and enter the same topic
4. Click "Send Test Notification" to verify

### Self-hosted ntfy

If you run your own ntfy server, set `NTFY_SERVER_URL` to your instance URL and provide an access token if authentication is required.

### Notification Types

| Status | Priority | When |
|---|---|---|
| Success | Default (3) | All shows synced without errors |
| Partial | High (4) | Some shows failed but others succeeded |
| Failed | Urgent (5) | Sync completely failed |

## API Reference

All endpoints are under `/api`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/health` | Health check with Sonarr connectivity |
| `GET` | `/api/version` | Version info |
| `GET` | `/api/stats` | Dashboard statistics |
| `GET` | `/api/mappings` | List all show mappings |
| `POST` | `/api/mappings` | Create mapping (auto-match or manual) |
| `PUT` | `/api/mappings/:id` | Update a mapping |
| `DELETE` | `/api/mappings/:id` | Delete a mapping |
| `GET` | `/api/mappings/:id/episodes` | Get episodes for a mapping |
| `POST` | `/api/sync` | Trigger manual sync |
| `GET` | `/api/logs` | Get sync history |
| `GET` | `/api/settings` | Get current settings |
| `PUT` | `/api/settings` | Update settings |
| `POST` | `/api/sonarr/test` | Test Sonarr connection |
| `POST` | `/api/ntfy/test` | Send test ntfy notification |
| `GET` | `/api/afl/search?q=` | Search AnimeFillerList |

## Development

```bash
# Frontend (hot reload)
cd web && npm run dev

# Backend
go run ./cmd/server --config config.yml
```

The Vite dev server proxies `/api` requests to the Go backend on port 8978.

## License

MIT
'@ | Set-Content -Path "README.md" -Encoding UTF8

# --- cmd/server/main.go ---
@'
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/config"
	"github.com/animefillerguard/animefillerguard/internal/database"
	"github.com/animefillerguard/animefillerguard/internal/sync"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Flags
	configPath := flag.String("config", "", "path to config YAML file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("AnimeFillerGuard %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().
		Str("version", version).
		Str("listen", cfg.ListenAddr).
		Str("sonarr", cfg.SonarrURL).
		Dur("scan_interval", cfg.ScanInterval).
		Msg("starting AnimeFillerGuard")

	// Initialize database
	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfg.DBPath).Msg("failed to open database")
	}
	defer db.Close()

	log.Info().Str("path", cfg.DBPath).Msg("database initialized")

	// Create application context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create core components
	syncEngine := sync.New(cfg, db)

	// Setup HTTP server with API routes
	router := setupRouter(cfg, db, syncEngine)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start scheduler in background
	go startScheduler(ctx, cfg, syncEngine)

	// Start HTTP server
	go func() {
		log.Info().Str("addr", cfg.ListenAddr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down...")
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}

	log.Info().Msg("AnimeFillerGuard stopped")
}

// startScheduler runs the periodic sync on the configured interval.
func startScheduler(ctx context.Context, cfg *config.Config, syncEngine *sync.Engine) {
	ticker := time.NewTicker(cfg.ScanInterval)
	defer ticker.Stop()

	// Run initial sync after a short delay
	time.Sleep(30 * time.Second)
	if err := syncEngine.RunSync(ctx, "scheduled"); err != nil {
		log.Error().Err(err).Msg("initial sync failed")
	}

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("scheduler stopped")
			return
		case <-ticker.C:
			if err := syncEngine.RunSync(ctx, "scheduled"); err != nil {
				log.Error().Err(err).Msg("scheduled sync failed")
			}
		}
	}
}
'@ | Set-Content -Path "cmd\server\main.go" -Encoding UTF8

# --- cmd/server/router.go ---
@'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/config"
	"github.com/animefillerguard/animefillerguard/internal/database"
	"github.com/animefillerguard/animefillerguard/internal/matcher"
	"github.com/animefillerguard/animefillerguard/internal/models"
	"github.com/animefillerguard/animefillerguard/internal/scraper"
	"github.com/animefillerguard/animefillerguard/internal/sonarr"
	"github.com/animefillerguard/animefillerguard/internal/notifier"
	"github.com/animefillerguard/animefillerguard/internal/sync"
	"github.com/animefillerguard/animefillerguard/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
)

// setupRouter creates the HTTP router with all API routes.
// setupRouter creates the HTTP router with all API routes.
func setupRouter(cfg *config.Config, db *database.DB, syncEngine *sync.Engine) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Health check
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			status := "ok"
			// Quick Sonarr ping
			if cfg.SonarrURL != "" {
				c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 30*time.Second)
				if _, err := c.TestConnection(r.Context()); err != nil {
					status = "degraded"
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":  status,
				"version": version,
			})
		})

		// Version info
		r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"version":    version,
				"commit":     commit,
				"build_date": buildDate,
			})
		})

		// Dashboard stats
		r.Get("/stats", handleGetStats(db))

		// Show mappings
		r.Route("/mappings", func(r chi.Router) {
			r.Get("/", handleGetMappings(db))
			r.Post("/", handleCreateMapping(cfg, db))
			r.Put("/{id}", handleUpdateMapping(db))
			r.Delete("/{id}", handleDeleteMapping(db))
			r.Get("/{id}/episodes", handleGetEpisodes(db))
		})

		// Sync operations
		r.Post("/sync", handleTriggerSync(syncEngine))

		// Sync logs
		r.Get("/logs", handleGetLogs(db))

		// Settings
		r.Get("/settings", handleGetSettings(cfg, db))
		r.Put("/settings", handleUpdateSettings(cfg, db))

		// Sonarr connection test
		r.Post("/sonarr/test", handleTestSonarr())

		// ntfy test notification
		r.Post("/ntfy/test", handleTestNtfy(cfg))

		// AFL search (proxy to avoid CORS)
		r.Get("/afl/search", handleAFLSearch())
	})

	// Serve embedded Svelte SPA for all non-API routes
	r.NotFound(web.SPAHandler().ServeHTTP)

	return r
}

// --- Handlers ---

func handleGetStats(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		canon, mixed, filler, total, err := db.GetFillerStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"canon":  canon,
			"mixed":  mixed,
			"filler": filler,
			"total":  total,
		})
	}
}

func handleGetMappings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mappings)
	}
}

func handleCreateMapping(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SonarrSeriesID int    `json:"sonarr_series_id"`
			AFLSlug        string `json:"afl_slug"`
			AutoMatch      bool   `json:"auto_match"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.SonarrSeriesID == 0 {
			writeError(w, http.StatusBadRequest, "sonarr_series_id is required")
			return
		}

		// If auto_match, find the best AFL match using the matcher
		if req.AutoMatch {
			// Get series info from Sonarr
			c := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, 30*time.Second)
			series, err := c.GetAnimeSeries(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to fetch Sonarr series: "+err.Error())
				return
			}

			var target *models.SonarrSeries
			for i, s := range series {
				if s.ID == req.SonarrSeriesID {
					target = &series[i]
					break
				}
			}
			if target == nil {
				writeError(w, http.StatusNotFound, "series not found in Sonarr")
				return
			}

			// Get AFL shows list
			scrp := scraper.New(30*time.Second, 2*time.Second, 24*time.Hour)
			aflShows, err := scrp.GetAllShows(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to fetch AFL shows: "+err.Error())
				return
			}

			// Run matcher
			m := matcher.New(0.7)
			matches := m.FindMatches(r.Context(), *target, aflShows)
			if len(matches) == 0 {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"auto_match": true,
					"found":      false,
					"message":    "No match found. Please search AFL manually.",
				})
				return
			}

			// Return top matches for user confirmation
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"auto_match": true,
				"found":      true,
				"matches":    matches,
			})
			return
		}

		// Manual mapping: sonarr_series_id + afl_slug
		if req.AFLSlug == "" {
			writeError(w, http.StatusBadRequest, "afl_slug is required for manual mapping")
			return
		}

		mapping := models.ShowMapping{
			SonarrSeriesID: req.SonarrSeriesID,
			AFLSlug:        req.AFLSlug,
			FillerAction:   "unmonitor",
			MixedAction:    "monitor",
			Enabled:        true,
		}

		if err := db.UpsertMapping(r.Context(), &mapping); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save mapping: "+err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, mapping)
	}
}

func handleUpdateMapping(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		// Get existing mapping
		mapping, err := getMappingByID(r.Context(), db, id)
		if err != nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}

		// Decode update fields
		var update struct {
			FillerAction  *string `json:"filler_action"`
			MixedAction   *string `json:"mixed_action"`
			EpisodeOffset *int    `json:"episode_offset"`
			Enabled       *bool   `json:"is_active"`
			AFLSlug       *string `json:"afl_slug"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if update.FillerAction != nil {
			mapping.FillerAction = *update.FillerAction
		}
		if update.MixedAction != nil {
			mapping.MixedAction = *update.MixedAction
		}
		if update.EpisodeOffset != nil {
			mapping.EpisodeOffset = *update.EpisodeOffset
		}
		if update.Enabled != nil {
			mapping.Enabled = *update.Enabled
		}
		if update.AFLSlug != nil {
			mapping.AFLSlug = *update.AFLSlug
		}

		if err := db.UpsertMapping(r.Context(), mapping); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update mapping: "+err.Error())
			return
		}

		writeJSON(w, http.StatusOK, mapping)
	}
}

func handleDeleteMapping(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		if err := db.DeleteMapping(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete mapping: "+err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func handleGetEpisodes(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}

		episodes, err := db.GetEpisodes(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, episodes)
	}
}

func handleTriggerSync(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		go func() {
			if err := engine.RunSync(r.Context(), "manual"); err != nil {
				log.Error().Err(err).Msg("manual sync failed")
			}
		}()
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync triggered"})
	}
}

func handleGetLogs(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
				limit = parsed
			}
		}
		logs, err := db.GetRecentSyncLogs(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}
}

func handleGetSettings(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return current config as settings (mask API key)
		maskedKey := ""
		if cfg.SonarrAPIKey != "" {
			maskedKey = cfg.SonarrAPIKey[:4] + "..." + cfg.SonarrAPIKey[len(cfg.SonarrAPIKey)-4:]
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sonarr_url":            cfg.SonarrURL,
			"sonarr_api_key_masked": maskedKey,
			"sonarr_api_key_set":    cfg.SonarrAPIKey != "",
			"default_filler_action": cfg.DefaultFillerAction,
			"default_mixed_canon_action":  cfg.DefaultMixedCanonAction,
			"scan_interval":         cfg.ScanInterval.String(),
			"enable_tagging":        cfg.EnableTagging,
			"cache_ttl":             cfg.CacheTTL.String(),
		})
	}
}

func handleUpdateSettings(cfg *config.Config, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var update struct {
			SonarrURL          *string `json:"sonarr_url"`
			SonarrAPIKey       *string `json:"sonarr_api_key"`
			DefaultFillerAction *string `json:"default_filler_action"`
			DefaultMixedCanonAction  *string `json:"default_mixed_canon_action"`
			ScanInterval       *string `json:"scan_interval"`
			EnableTagging      *bool   `json:"enable_tagging"`
			CacheTTL           *string `json:"cache_ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		// Apply updates to live config
		if update.SonarrURL != nil {
			cfg.SonarrURL = *update.SonarrURL
		}
		if update.SonarrAPIKey != nil && *update.SonarrAPIKey != "" {
			cfg.SonarrAPIKey = *update.SonarrAPIKey
		}
		if update.DefaultFillerAction != nil {
			cfg.DefaultFillerAction = *update.DefaultFillerAction
		}
		if update.DefaultMixedCanonAction != nil {
			cfg.DefaultMixedCanonAction = *update.DefaultMixedCanonAction
		}
		if update.EnableTagging != nil {
			cfg.EnableTagging = *update.EnableTagging
		}
		// Note: sync_interval and cache_ttl require restart to take effect
		// In a production version, these would trigger scheduler reconfiguration

		// Persist settings to DB for survival across restarts
		for k, v := range map[string]string{
			"sonarr_url":           cfg.SonarrURL,
			"default_filler_action": string(cfg.DefaultFillerAction),
			"default_mixed_canon_action": string(cfg.DefaultMixedCanonAction),
		} {
			if err := db.SetSetting(r.Context(), k, v); err != nil {
				log.Warn().Err(err).Str("key", k).Msg("failed to persist setting")
			}
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "settings updated"})
	}
}

func handleTestSonarr() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL    string `json:"url"`
			APIKey string `json:"api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.URL == "" || req.APIKey == "" {
			writeError(w, http.StatusBadRequest, "url and api_key are required")
			return
		}

		c := sonarr.New(req.URL, req.APIKey, 10*time.Second)
		info, err := c.TestConnection(r.Context())
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"version": info,
		})
	}
}

func handleAFLSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			writeError(w, http.StatusBadRequest, "q parameter is required")
			return
		}

		scrp := scraper.New(30*time.Second, 2*time.Second, 24*time.Hour)
		results, err := scrp.SearchShows(r.Context(), query)
		if err != nil {
			writeError(w, http.StatusBadGateway, "AFL search failed: "+err.Error())
			return
		}

		writeJSON(w, http.StatusOK, results)
	}
}


// getMappingByID finds a mapping by its primary key using GetAllMappings.
func getMappingByID(ctx context.Context, db *database.DB, id int64) (*models.ShowMapping, error) {
	mappings, err := db.GetAllMappings(ctx)
	if err != nil {
		return nil, err
	}
	for i := range mappings {
		if mappings[i].ID == id {
			return &mappings[i], nil
		}
	}
	return nil, fmt.Errorf("mapping %d not found", id)
}

func handleTestNtfy(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.NtfyEnabled || cfg.NtfyServerURL == "" || cfg.NtfyTopic == "" {
			writeError(w, http.StatusBadRequest, "ntfy is not configured")
			return
		}
		n := notifier.New(cfg.NtfyServerURL, cfg.NtfyTopic, cfg.NtfyToken)
		result := notifier.SyncResult{Status: "success", Trigger: "test"}
		if err := n.SendSyncResult(r.Context(), result); err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "test notification sent"})
	}
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
'@ | Set-Content -Path "cmd\server\router.go" -Encoding UTF8

# --- docker-compose.yml ---
@'
# =============================================================================
# AnimeFillerGuard - Docker Compose Example
# Add this to your existing *arr stack docker-compose.yml
# =============================================================================

version: "3.8"

services:
  animefillerguard:
    build: .
    # Or use the published image:
    # image: ghcr.io/animefillerguard/animefillerguard:latest
    container_name: animefillerguard
    restart: unless-stopped
    ports:
      - "8978:8978"
    environment:
      - SONARR_URL=http://sonarr:8989   # Use container name if on same network
      - SONARR_API_KEY=your-api-key-here
      - SCAN_INTERVAL=6h
      - LOG_LEVEL=info
      - TZ=America/New_York              # Your timezone
    volumes:
      - ./config/animefillerguard:/config
    networks:
      - arr-network
    depends_on:
      - sonarr

  # --- Your existing *arr services (example) ---
  # sonarr:
  #   image: lscr.io/linuxserver/sonarr:latest
  #   container_name: sonarr
  #   environment:
  #     - PUID=1000
  #     - PGID=1000
  #     - TZ=America/New_York
  #   volumes:
  #     - ./config/sonarr:/config
  #     - /path/to/media:/media
  #   ports:
  #     - "8989:8989"
  #   restart: unless-stopped
  #   networks:
  #     - arr-network

networks:
  arr-network:
    driver: bridge
'@ | Set-Content -Path "docker-compose.yml" -Encoding UTF8

# --- go.mod ---
@'
module github.com/animefillerguard/animefillerguard

go 1.22

require (
	github.com/PuerkitoBio/goquery v1.9.1
	github.com/go-chi/chi/v5 v5.0.12
	github.com/go-chi/cors v1.2.1
	github.com/rs/zerolog v1.32.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.29.5
)
'@ | Set-Content -Path "go.mod" -Encoding UTF8

# --- internal/config/config.go ---
@'
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	// Sonarr connection
	SonarrURL    string `yaml:"sonarr_url"`
	SonarrAPIKey string `yaml:"sonarr_api_key"`

	// Scheduler
	ScanInterval time.Duration `yaml:"scan_interval"`

	// Server
	ListenAddr string `yaml:"listen_addr"`
	BaseURL    string `yaml:"base_url"`

	// Database
	DBPath string `yaml:"db_path"`

	// Scraper
	CacheTTL       time.Duration `yaml:"cache_ttl"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	RateLimitDelay time.Duration `yaml:"rate_limit_delay"`

	// Defaults
	DefaultFillerAction       FillerAction `yaml:"default_filler_action"`
	DefaultMixedCanonAction   FillerAction `yaml:"default_mixed_canon_action"`
	EnableTagging             bool         `yaml:"enable_tagging"`
	FillerTagName             string       `yaml:"filler_tag_name"`

	// Notifications
	NtfyEnabled   bool   `yaml:"ntfy_enabled"`
	NtfyServerURL string `yaml:"ntfy_server_url"`
	NtfyTopic     string `yaml:"ntfy_topic"`
	NtfyToken     string `yaml:"ntfy_token"`

	// Logging
	LogLevel string `yaml:"log_level"`
}

// FillerAction defines what to do with filler episodes.
type FillerAction string

const (
	ActionUnmonitor FillerAction = "unmonitor"
	ActionMonitor   FillerAction = "monitor"
	ActionSkip      FillerAction = "skip" // unmonitor + delete if exists
)

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		SonarrURL:               "http://localhost:8989",
		SonarrAPIKey:            "",
		ScanInterval:            6 * time.Hour,
		ListenAddr:              ":8978",
		BaseURL:                 "/",
		DBPath:                  "/config/animefillerguard.db",
		CacheTTL:                24 * time.Hour,
		RequestTimeout:          30 * time.Second,
		RateLimitDelay:          2 * time.Second,
		DefaultFillerAction:     ActionUnmonitor,
		DefaultMixedCanonAction: ActionMonitor,
		EnableTagging:           false,
		FillerTagName:           "filler",
		NtfyEnabled:             false,
		NtfyServerURL:           "https://ntfy.sh",
		NtfyTopic:               "",
		NtfyToken:               "",
		LogLevel:                "info",
	}
}

// Load reads configuration from a YAML file, then overlays environment variables.
// Environment variables take precedence over the YAML file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try loading YAML config file
	if path != "" {
		if err := cfg.loadYAML(path); err != nil {
			// Only error if file was explicitly specified and doesn't exist
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("loading config file: %w", err)
			}
		}
	}

	// Overlay environment variables
	cfg.loadEnv()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func (c *Config) loadYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

func (c *Config) loadEnv() {
	if v := os.Getenv("SONARR_URL"); v != "" {
		c.SonarrURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("SONARR_API_KEY"); v != "" {
		c.SonarrAPIKey = v
	}
	if v := os.Getenv("SCAN_INTERVAL"); v != "" {
		if d, err := parseDuration(v); err == nil {
			c.ScanInterval = d
		}
	}
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		c.ListenAddr = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if d, err := parseDuration(v); err == nil {
			c.CacheTTL = d
		}
	}
	if v := os.Getenv("DEFAULT_FILLER_ACTION"); v != "" {
		c.DefaultFillerAction = FillerAction(v)
	}
	if v := os.Getenv("DEFAULT_MIXED_CANON_ACTION"); v != "" {
		c.DefaultMixedCanonAction = FillerAction(v)
	}
	if v := os.Getenv("ENABLE_TAGGING"); v != "" {
		c.EnableTagging = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("FILLER_TAG_NAME"); v != "" {
		c.FillerTagName = v
	}
	if v := os.Getenv("NTFY_ENABLED"); v != "" {
		c.NtfyEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("NTFY_SERVER_URL"); v != "" {
		c.NtfyServerURL = v
	}
	if v := os.Getenv("NTFY_TOPIC"); v != "" {
		c.NtfyTopic = v
	}
	if v := os.Getenv("NTFY_TOKEN"); v != "" {
		c.NtfyToken = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
}

func (c *Config) validate() error {
	if c.SonarrURL == "" {
		return fmt.Errorf("sonarr_url is required")
	}
	if c.ScanInterval < 1*time.Minute {
		return fmt.Errorf("scan_interval must be at least 1 minute")
	}
	if c.CacheTTL < 1*time.Hour {
		return fmt.Errorf("cache_ttl must be at least 1 hour")
	}
	switch c.DefaultFillerAction {
	case ActionUnmonitor, ActionMonitor, ActionSkip:
		// valid
	default:
		return fmt.Errorf("invalid default_filler_action: %s", c.DefaultFillerAction)
	}
	switch c.DefaultMixedCanonAction {
	case ActionUnmonitor, ActionMonitor, ActionSkip:
		// valid
	default:
		return fmt.Errorf("invalid default_mixed_canon_action: %s", c.DefaultMixedCanonAction)
	}
	return nil
}

// parseDuration handles both Go duration strings ("6h") and plain integers ("360" = minutes).
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if mins, err := strconv.Atoi(s); err == nil {
		return time.Duration(mins) * time.Minute, nil
	}
	return 0, fmt.Errorf("invalid duration: %s", s)
}
'@ | Set-Content -Path "internal\config\config.go" -Encoding UTF8

# --- internal/database/database.go ---
@'
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/models"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// New opens or creates a SQLite database at the given path.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Set connection pool for SQLite (single writer)
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs schema migrations.
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS show_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sonarr_series_id INTEGER NOT NULL UNIQUE,
			sonarr_title TEXT NOT NULL,
			afl_slug TEXT NOT NULL,
			afl_title TEXT NOT NULL,
			auto_matched INTEGER NOT NULL DEFAULT 1,
			match_confidence REAL NOT NULL DEFAULT 0.0,
			filler_action TEXT NOT NULL DEFAULT 'unmonitor',
			mixed_action TEXT NOT NULL DEFAULT 'monitor',
			enabled INTEGER NOT NULL DEFAULT 1,
			episode_offset INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS episode_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mapping_id INTEGER NOT NULL,
			episode_num INTEGER NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'unknown',
			arc_name TEXT NOT NULL DEFAULT '',
			last_scraped DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (mapping_id) REFERENCES show_mappings(id) ON DELETE CASCADE,
			UNIQUE(mapping_id, episode_num)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			status TEXT NOT NULL DEFAULT 'running',
			shows_processed INTEGER NOT NULL DEFAULT 0,
			episodes_updated INTEGER NOT NULL DEFAULT 0,
			errors TEXT NOT NULL DEFAULT '',
			trigger_type TEXT NOT NULL DEFAULT 'manual'
		)`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_episode_info_mapping ON episode_info(mapping_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_logs_started ON sync_logs(started_at DESC)`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return nil
}

// --- Show Mappings ---

// GetAllMappings returns all show mappings.
func (db *DB) GetAllMappings(ctx context.Context) ([]models.ShowMapping, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
		       auto_matched, match_confidence, filler_action, mixed_action,
		       enabled, episode_offset, created_at, updated_at
		FROM show_mappings ORDER BY sonarr_title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []models.ShowMapping
	for rows.Next() {
		var m models.ShowMapping
		err := rows.Scan(
			&m.ID, &m.SonarrSeriesID, &m.SonarrTitle, &m.AFLSlug, &m.AFLTitle,
			&m.AutoMatched, &m.MatchConfidence, &m.FillerAction, &m.MixedAction,
			&m.Enabled, &m.EpisodeOffset, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// GetMappingBySonarrID retrieves a mapping by Sonarr series ID.
func (db *DB) GetMappingBySonarrID(ctx context.Context, seriesID int) (*models.ShowMapping, error) {
	var m models.ShowMapping
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, sonarr_series_id, sonarr_title, afl_slug, afl_title,
		       auto_matched, match_confidence, filler_action, mixed_action,
		       enabled, episode_offset, created_at, updated_at
		FROM show_mappings WHERE sonarr_series_id = ?`, seriesID).Scan(
		&m.ID, &m.SonarrSeriesID, &m.SonarrTitle, &m.AFLSlug, &m.AFLTitle,
		&m.AutoMatched, &m.MatchConfidence, &m.FillerAction, &m.MixedAction,
		&m.Enabled, &m.EpisodeOffset, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpsertMapping inserts or updates a show mapping.
func (db *DB) UpsertMapping(ctx context.Context, m *models.ShowMapping) error {
	now := time.Now()
	m.UpdatedAt = now

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO show_mappings (sonarr_series_id, sonarr_title, afl_slug, afl_title,
		    auto_matched, match_confidence, filler_action, mixed_action,
		    enabled, episode_offset, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(sonarr_series_id) DO UPDATE SET
		    sonarr_title = excluded.sonarr_title,
		    afl_slug = excluded.afl_slug,
		    afl_title = excluded.afl_title,
		    auto_matched = excluded.auto_matched,
		    match_confidence = excluded.match_confidence,
		    filler_action = excluded.filler_action,
		    mixed_action = excluded.mixed_action,
		    enabled = excluded.enabled,
		    episode_offset = excluded.episode_offset,
		    updated_at = excluded.updated_at`,
		m.SonarrSeriesID, m.SonarrTitle, m.AFLSlug, m.AFLTitle,
		m.AutoMatched, m.MatchConfidence, m.FillerAction, m.MixedAction,
		m.Enabled, m.EpisodeOffset, now, now,
	)
	return err
}

// DeleteMapping removes a mapping and its episode info.
func (db *DB) DeleteMapping(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM show_mappings WHERE id = ?`, id)
	return err
}

// --- Episode Info ---

// GetEpisodes returns all episode info for a mapping.
func (db *DB) GetEpisodes(ctx context.Context, mappingID int64) ([]models.EpisodeInfo, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, mapping_id, episode_num, title, type, arc_name, last_scraped
		FROM episode_info WHERE mapping_id = ? ORDER BY episode_num ASC`, mappingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []models.EpisodeInfo
	for rows.Next() {
		var e models.EpisodeInfo
		err := rows.Scan(&e.ID, &e.MappingID, &e.EpisodeNum, &e.Title, &e.Type, &e.ArcName, &e.LastScraped)
		if err != nil {
			return nil, err
		}
		episodes = append(episodes, e)
	}
	return episodes, rows.Err()
}

// BulkUpsertEpisodes inserts or updates episodes for a mapping.
func (db *DB) BulkUpsertEpisodes(ctx context.Context, episodes []models.EpisodeInfo) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO episode_info (mapping_id, episode_num, title, type, arc_name, last_scraped)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(mapping_id, episode_num) DO UPDATE SET
		    title = excluded.title,
		    type = excluded.type,
		    arc_name = excluded.arc_name,
		    last_scraped = excluded.last_scraped`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, e := range episodes {
		_, err := stmt.ExecContext(ctx, e.MappingID, e.EpisodeNum, e.Title, e.Type, e.ArcName, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// --- Sync Logs ---

// CreateSyncLog starts a new sync log entry.
func (db *DB) CreateSyncLog(ctx context.Context, triggerType string) (int64, error) {
	result, err := db.conn.ExecContext(ctx, `
		INSERT INTO sync_logs (trigger_type) VALUES (?)`, triggerType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// CompleteSyncLog finishes a sync log entry.
func (db *DB) CompleteSyncLog(ctx context.Context, id int64, status string, showsProcessed, episodesUpdated int, errors string) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE sync_logs SET completed_at = ?, status = ?, shows_processed = ?,
		    episodes_updated = ?, errors = ?
		WHERE id = ?`, time.Now(), status, showsProcessed, episodesUpdated, errors, id)
	return err
}

// GetRecentSyncLogs returns the most recent sync logs.
func (db *DB) GetRecentSyncLogs(ctx context.Context, limit int) ([]models.SyncLog, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, started_at, completed_at, status, shows_processed,
		       episodes_updated, errors, trigger_type
		FROM sync_logs ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.SyncLog
	for rows.Next() {
		var l models.SyncLog
		var completedAt sql.NullTime
		err := rows.Scan(&l.ID, &l.StartedAt, &completedAt, &l.Status,
			&l.ShowsProcessed, &l.EpisodesUpdated, &l.Errors, &l.Trigger)
		if err != nil {
			return nil, err
		}
		if completedAt.Valid {
			l.CompletedAt = completedAt.Time
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- App Settings ---

// GetSetting retrieves a setting value.
func (db *DB) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := db.conn.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting stores a setting value.
func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// GetFillerStats returns aggregate filler statistics.
func (db *DB) GetFillerStats(ctx context.Context) (canon, mixed, filler, total int, err error) {
	err = db.conn.QueryRowContext(ctx, `
		SELECT
			COUNT(CASE WHEN type = 'canon' THEN 1 END),
			COUNT(CASE WHEN type = 'mixed_canon' THEN 1 END),
			COUNT(CASE WHEN type = 'filler' THEN 1 END),
			COUNT(*)
		FROM episode_info`).Scan(&canon, &mixed, &filler, &total)
	return
}
'@ | Set-Content -Path "internal\database\database.go" -Encoding UTF8

# --- internal/matcher/matcher.go ---
@'
package matcher

import (
	"context"
	"strings"
	"unicode"

	"github.com/animefillerguard/animefillerguard/internal/models"
	"github.com/rs/zerolog/log"
)

// MatchResult represents a potential match between a Sonarr series and an AFL show.
type MatchResult struct {
	AFLShow    models.AFLShow `json:"afl_show"`
	Confidence float64        `json:"confidence"` // 0.0 - 1.0
	Method     string         `json:"method"`     // "exact", "normalized", "fuzzy", "alternate"
}

// Matcher handles matching Sonarr series to AnimeFillerList shows.
type Matcher struct {
	minConfidence float64
}

// New creates a new Matcher with the given minimum confidence threshold.
func New(minConfidence float64) *Matcher {
	if minConfidence <= 0 {
		minConfidence = 0.6
	}
	return &Matcher{minConfidence: minConfidence}
}

// FindMatches returns ranked match candidates for a Sonarr series against the AFL show list.
func (m *Matcher) FindMatches(ctx context.Context, series models.SonarrSeries, aflShows []models.AFLShow) []MatchResult {
	var results []MatchResult

	// Collect all title variants from Sonarr (main + alternates)
	sonarrTitles := []string{series.Title}
	for _, alt := range series.AlternateTitles {
		if alt.Title != "" {
			sonarrTitles = append(sonarrTitles, alt.Title)
		}
	}

	for _, aflShow := range aflShows {
		best := MatchResult{AFLShow: aflShow}

		for _, sonarrTitle := range sonarrTitles {
			// 1. Exact match (case-insensitive)
			if strings.EqualFold(sonarrTitle, aflShow.Title) {
				best = MatchResult{AFLShow: aflShow, Confidence: 1.0, Method: "exact"}
				break
			}

			// 2. Normalized match (strip punctuation, articles, whitespace)
			normSonarr := normalize(sonarrTitle)
			normAFL := normalize(aflShow.Title)
			if normSonarr == normAFL {
				if best.Confidence < 0.95 {
					best = MatchResult{AFLShow: aflShow, Confidence: 0.95, Method: "normalized"}
				}
				continue
			}

			// 3. Containment check (one title contains the other)
			containScore := containmentScore(normSonarr, normAFL)
			if containScore > best.Confidence {
				best = MatchResult{AFLShow: aflShow, Confidence: containScore, Method: "containment"}
			}

			// 4. Fuzzy match (Jaro-Winkler similarity)
			jw := jaroWinkler(normSonarr, normAFL)
			if jw > best.Confidence {
				best = MatchResult{AFLShow: aflShow, Confidence: jw, Method: "fuzzy"}
			}
		}

		if best.Confidence >= m.minConfidence {
			results = append(results, best)
		}
	}

	// Sort by confidence descending
	sortResults(results)

	// Cap at top 10 results
	if len(results) > 10 {
		results = results[:10]
	}

	log.Debug().
		Str("sonarr_title", series.Title).
		Int("candidates", len(results)).
		Msg("match results")

	return results
}

// BestMatch returns the single best match if it exceeds the confidence threshold.
func (m *Matcher) BestMatch(ctx context.Context, series models.SonarrSeries, aflShows []models.AFLShow) *MatchResult {
	matches := m.FindMatches(ctx, series, aflShows)
	if len(matches) == 0 {
		return nil
	}
	if matches[0].Confidence >= m.minConfidence {
		return &matches[0]
	}
	return nil
}

// MapEpisodeNumber maps a Sonarr episode to an AFL episode number,
// accounting for the episode offset and absolute vs seasonal numbering.
func MapEpisodeNumber(sonarrEp models.SonarrEpisode, offset int) int {
	// Prefer absolute episode number for anime (most accurate for AFL mapping)
	if sonarrEp.AbsoluteEpisodeNumber != nil && *sonarrEp.AbsoluteEpisodeNumber > 0 {
		return *sonarrEp.AbsoluteEpisodeNumber + offset
	}
	// Fallback: use season-relative number (less reliable for long-running anime)
	return sonarrEp.EpisodeNumber + offset
}

// --- String Normalization ---

// normalize reduces a title to a comparable form:
// - lowercase
// - strip punctuation and special chars
// - remove common articles/noise words
// - collapse whitespace
func normalize(s string) string {
	s = strings.ToLower(s)

	// Replace common separators with space
	for _, sep := range []string{"-", "_", ":", "~", ".", ",", "!", "?", "'", "\"", "(", ")"} {
		s = strings.ReplaceAll(s, sep, " ")
	}

	// Remove non-alphanumeric chars (keep spaces)
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			sb.WriteRune(r)
		}
	}
	s = sb.String()

	// Remove common noise words
	noise := []string{" the ", " a ", " an ", " no ", " wa ", " ga ", " wo "}
	for _, n := range noise {
		s = strings.ReplaceAll(" "+s+" ", n, " ")
	}

	// Collapse whitespace
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// containmentScore returns a confidence score based on substring containment.
// If one normalized title fully contains the other, it's likely a match.
func containmentScore(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	if strings.Contains(a, b) {
		// b is fully contained in a -- score based on length ratio
		return float64(len(b)) / float64(len(a)) * 0.9
	}
	if strings.Contains(b, a) {
		return float64(len(a)) / float64(len(b)) * 0.9
	}

	return 0
}

// --- Jaro-Winkler Similarity ---

// jaroWinkler computes Jaro-Winkler similarity between two strings.
// Returns a value between 0.0 (no match) and 1.0 (perfect match).
func jaroWinkler(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Jaro distance
	jaro := jaroDistance(s1, s2)

	// Winkler modification: boost score for common prefix (up to 4 chars)
	prefixLen := 0
	for i := 0; i < len(s1) && i < len(s2) && i < 4; i++ {
		if s1[i] == s2[i] {
			prefixLen++
		} else {
			break
		}
	}

	p := 0.1 // Winkler scaling factor
	return jaro + float64(prefixLen)*p*(1.0-jaro)
}

func jaroDistance(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	l1, l2 := len(s1), len(s2)
	if l1 == 0 || l2 == 0 {
		return 0.0
	}

	// Max distance for matching characters
	matchDist := 0
	if l1 > l2 {
		matchDist = l1/2 - 1
	} else {
		matchDist = l2/2 - 1
	}
	if matchDist < 0 {
		matchDist = 0
	}

	s1Matches := make([]bool, l1)
	s2Matches := make([]bool, l2)

	matches := 0
	transpositions := 0

	// Find matching characters
	for i := 0; i < l1; i++ {
		start := i - matchDist
		if start < 0 {
			start = 0
		}
		end := i + matchDist + 1
		if end > l2 {
			end = l2
		}

		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	// Count transpositions
	k := 0
	for i := 0; i < l1; i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}

	m := float64(matches)
	return (m/float64(l1) + m/float64(l2) + (m-float64(transpositions)/2.0)/m) / 3.0
}

// --- Sorting ---

func sortResults(results []MatchResult) {
	// Simple insertion sort (small N, typically < 10 candidates above threshold)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Confidence > results[j-1].Confidence; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
'@ | Set-Content -Path "internal\matcher\matcher.go" -Encoding UTF8

# --- internal/matcher/matcher_test.go ---
@'
package matcher

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Naruto: Shippuden", "naruto shippuden"},
		{"One Piece (TV)", "one piece tv"},
		{"Bleach: Thousand-Year Blood War", "bleach thousand year blood war"},
		{"DEATH NOTE", "death note"},
		{"  extra   spaces  ", "extra spaces"},
	}

	m := New()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := m.Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJaroWinkler(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
	}{
		{"naruto", "naruto", 1.0},
		{"naruto shippuden", "naruto shippuuden", 0.9},
		{"one piece", "one peace", 0.85},
		{"completely different", "naruto", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			score := JaroWinkler(tt.a, tt.b)
			if score < tt.min {
				t.Errorf("JaroWinkler(%q, %q) = %f, want >= %f", tt.a, tt.b, score, tt.min)
			}
		})
	}
}
'@ | Set-Content -Path "internal\matcher\matcher_test.go" -Encoding UTF8

# --- internal/models/models.go ---
@'
package models

import "time"

// EpisodeType classifies an episode based on AnimeFillerList data.
type EpisodeType string

const (
	EpisodeCanon      EpisodeType = "canon"
	EpisodeMixedCanon EpisodeType = "mixed_canon"
	EpisodeFiller     EpisodeType = "filler"
	EpisodeUnknown    EpisodeType = "unknown"
)

// FillerAction defines what to do with filler/mixed episodes.
type FillerAction string

const (
	ActionUnmonitor FillerAction = "unmonitor"
	ActionMonitor   FillerAction = "monitor"
	ActionSkip      FillerAction = "skip"
)

// ShowMapping links a Sonarr series to an AnimeFillerList show.
type ShowMapping struct {
	ID              int64        `json:"id"`
	SonarrSeriesID  int          `json:"sonarr_series_id"`
	SonarrTitle     string       `json:"sonarr_title"`
	AFLSlug         string       `json:"afl_slug"` // e.g., "naruto-shippuden"
	AFLTitle        string       `json:"afl_title"`
	AutoMatched     bool         `json:"auto_matched"`
	MatchConfidence float64      `json:"match_confidence"` // 0.0 - 1.0
	FillerAction    FillerAction `json:"filler_action"`    // per-show override
	MixedAction     FillerAction `json:"mixed_action"`     // per-show override
	Enabled         bool         `json:"enabled"`
	EpisodeOffset   int          `json:"episode_offset"` // offset for numbering mismatches
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// EpisodeInfo holds filler data for a single episode.
type EpisodeInfo struct {
	ID          int64       `json:"id"`
	MappingID   int64       `json:"mapping_id"`
	EpisodeNum  int         `json:"episode_num"` // AFL absolute episode number
	Title       string      `json:"title"`
	Type        EpisodeType `json:"type"`
	ArcName     string      `json:"arc_name,omitempty"`
	LastScraped time.Time   `json:"last_scraped"`
}

// SyncLog records each sync operation.
type SyncLog struct {
	ID              int64     `json:"id"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	Status          string    `json:"status"` // "success", "partial", "failed"
	ShowsProcessed  int       `json:"shows_processed"`
	EpisodesUpdated int       `json:"episodes_updated"`
	Errors          string    `json:"errors,omitempty"`
	Trigger         string    `json:"trigger"` // "scheduled", "manual"
}

// AppSettings stores user-configurable settings in the DB.
type AppSettings struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// --- Sonarr API Types ---

// SonarrSeries represents a series from the Sonarr API.
type SonarrSeries struct {
	ID             int           `json:"id"`
	Title          string        `json:"title"`
	SortTitle      string        `json:"sortTitle"`
	SeriesType     string        `json:"seriesType"` // "anime", "standard", etc.
	TVDBID         int           `json:"tvdbId"`
	IMDBID         string        `json:"imdbId"`
	Path           string        `json:"path"`
	Monitored      bool          `json:"monitored"`
	SeasonCount    int           `json:"seasonCount"`
	Tags           []int         `json:"tags"`
	Images         []SonarrImage `json:"images"`
	Statistics     *SonarrStats  `json:"statistics,omitempty"`
	AlternateTitles []SonarrAltTitle `json:"alternateTitles,omitempty"`
}

// SonarrEpisode represents an episode from the Sonarr API.
type SonarrEpisode struct {
	ID                  int    `json:"id"`
	SeriesID            int    `json:"seriesId"`
	SeasonNumber        int    `json:"seasonNumber"`
	EpisodeNumber       int    `json:"episodeNumber"`
	AbsoluteEpisodeNumber *int `json:"absoluteEpisodeNumber,omitempty"`
	Title               string `json:"title"`
	Monitored           bool   `json:"monitored"`
	HasFile             bool   `json:"hasFile"`
}

// SonarrTag represents a tag in Sonarr.
type SonarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// SonarrImage represents a series image.
type SonarrImage struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

// SonarrStats holds series statistics.
type SonarrStats struct {
	EpisodeFileCount int `json:"episodeFileCount"`
	EpisodeCount     int `json:"episodeCount"`
	TotalEpisodeCount int `json:"totalEpisodeCount"`
}

// SonarrAltTitle holds alternate title info.
type SonarrAltTitle struct {
	Title string `json:"title"`
}

// --- AFL Types ---

// AFLShow represents a show listing from AnimeFillerList.
type AFLShow struct {
	Title string `json:"title"`
	Slug  string `json:"slug"` // URL path segment
	URL   string `json:"url"`
}

// AFLEpisode represents episode filler data from AnimeFillerList.
type AFLEpisode struct {
	Number int         `json:"number"`
	Title  string      `json:"title"`
	Type   EpisodeType `json:"type"`
}
'@ | Set-Content -Path "internal\models\models.go" -Encoding UTF8

# --- internal/notifier/ntfy.go ---
@'
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SyncResult holds the data needed to format a notification.
type SyncResult struct {
	Status          string // "success", "partial", "failed"
	ShowsProcessed  int
	EpisodesUpdated int
	Errors          []string
	Duration        time.Duration
	Trigger         string // "scheduled", "manual"
}

// Notifier sends notifications via ntfy.sh.
type Notifier struct {
	serverURL  string // e.g., "https://ntfy.sh"
	topic      string
	token      string // optional access token
	httpClient *http.Client
}

// New creates a new ntfy Notifier.
func New(serverURL, topic, token string) *Notifier {
	return &Notifier{
		serverURL: serverURL,
		topic:     topic,
		token:     token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enabled returns true if the notifier is configured (has server URL and topic).
func (n *Notifier) Enabled() bool {
	return n.serverURL != "" && n.topic != ""
}

// UpdateConfig updates the notifier's configuration dynamically.
func (n *Notifier) UpdateConfig(serverURL, topic, token string) {
	n.serverURL = serverURL
	n.topic = topic
	n.token = token
}

// SendSyncResult sends a formatted notification about a sync result.
func (n *Notifier) SendSyncResult(ctx context.Context, result SyncResult) error {
	if !n.Enabled() {
		return nil
	}

	title, body, priority, tags := formatSyncMessage(result)

	return n.send(ctx, title, body, priority, tags)
}

// SendTestNotification sends a test message to verify the configuration.
func (n *Notifier) SendTestNotification(ctx context.Context) error {
	if !n.Enabled() {
		return fmt.Errorf("ntfy is not configured (missing server URL or topic)")
	}

	return n.send(ctx,
		"AnimeFillerGuard - Test",
		"If you see this, notifications are working correctly!",
		3, // default priority
		[]string{"white_check_mark", "test_tube"},
	)
}

// send publishes a message to the ntfy topic.
func (n *Notifier) send(ctx context.Context, title, body string, priority int, tags []string) error {
	payload := map[string]interface{}{
		"topic":    n.topic,
		"title":    title,
		"message":  body,
		"priority": priority,
		"tags":     tags,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling ntfy payload: %w", err)
	}

	url := fmt.Sprintf("%s", n.serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("creating ntfy request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if n.token != "" {
		req.Header.Set("Authorization", "Bearer "+n.token)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	return nil
}

// formatSyncMessage creates a human-readable notification from sync results.
func formatSyncMessage(r SyncResult) (title, body string, priority int, tags []string) {
	switch r.Status {
	case "success":
		title = "Sync Complete"
		tags = []string{"white_check_mark", "tv"}
		priority = 3 // default
	case "partial":
		title = "Sync Completed with Errors"
		tags = []string{"warning", "tv"}
		priority = 4 // high
	case "failed":
		title = "Sync Failed"
		tags = []string{"x", "tv"}
		priority = 5 // urgent
	default:
		title = "Sync Update"
		tags = []string{"tv"}
		priority = 3
	}

	body = fmt.Sprintf("Shows processed: %d\nEpisodes updated: %d\nDuration: %s\nTrigger: %s",
		r.ShowsProcessed, r.EpisodesUpdated, r.Duration.Round(time.Second), r.Trigger)

	if len(r.Errors) > 0 {
		body += fmt.Sprintf("\n\nErrors (%d):", len(r.Errors))
		for i, e := range r.Errors {
			if i >= 5 {
				body += fmt.Sprintf("\n  ... and %d more", len(r.Errors)-5)
				break
			}
			body += fmt.Sprintf("\n  - %s", e)
		}
	}

	return
}
'@ | Set-Content -Path "internal\notifier\ntfy.go" -Encoding UTF8

# --- internal/notifier/ntfy_test.go ---
@'
package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNotifierEnabled(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		topic     string
		want      bool
	}{
		{"both set", "https://ntfy.sh", "test-topic", true},
		{"missing topic", "https://ntfy.sh", "", false},
		{"missing server", "", "test-topic", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(tt.serverURL, tt.topic, "")
			if got := n.Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSendSyncResult(t *testing.T) {
	var received map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("expected Authorization header")
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "test123"})
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "test-token")

	result := SyncResult{
		Status:          "success",
		ShowsProcessed:  5,
		EpisodesUpdated: 42,
		Duration:        3 * time.Second,
		Trigger:         "manual",
	}

	err := n.SendSyncResult(context.Background(), result)
	if err != nil {
		t.Fatalf("SendSyncResult() error = %v", err)
	}

	if received["topic"] != "test-topic" {
		t.Errorf("expected topic 'test-topic', got %v", received["topic"])
	}
	if received["title"] != "Sync Complete" {
		t.Errorf("expected title 'Sync Complete', got %v", received["title"])
	}
}

func TestSendSyncResultPartial(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		// Priority should be 4 (high) for partial
		if payload["priority"].(float64) != 4 {
			t.Errorf("expected priority 4 for partial, got %v", payload["priority"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "test456"})
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "")

	result := SyncResult{
		Status:          "partial",
		ShowsProcessed:  3,
		EpisodesUpdated: 10,
		Errors:          []string{"failed to process Naruto"},
		Duration:        5 * time.Second,
		Trigger:         "scheduled",
	}

	err := n.SendSyncResult(context.Background(), result)
	if err != nil {
		t.Fatalf("SendSyncResult() error = %v", err)
	}
}

func TestSendNotificationDisabled(t *testing.T) {
	n := New("", "", "")

	result := SyncResult{Status: "success"}
	err := n.SendSyncResult(context.Background(), result)
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
}

func TestSendNotificationHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "bad-token")

	err := n.SendTestNotification(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestFormatSyncMessage(t *testing.T) {
	tests := []struct {
		status       string
		wantTitle    string
		wantPriority int
	}{
		{"success", "Sync Complete", 3},
		{"partial", "Sync Completed with Errors", 4},
		{"failed", "Sync Failed", 5},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			title, _, priority, _ := formatSyncMessage(SyncResult{Status: tt.status})
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if priority != tt.wantPriority {
				t.Errorf("priority = %d, want %d", priority, tt.wantPriority)
			}
		})
	}
}
'@ | Set-Content -Path "internal\notifier\ntfy_test.go" -Encoding UTF8

# --- internal/scraper/scraper.go ---
@'
package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/animefillerguard/animefillerguard/internal/models"
	"github.com/rs/zerolog/log"
)

const (
	baseURL   = "https://www.animefillerlist.com"
	userAgent = "AnimeFillerGuard/1.0 (+https://github.com/animefillerguard/animefillerguard)"
)

// Scraper handles fetching and parsing data from AnimeFillerList.com.
type Scraper struct {
	client         *http.Client
	rateLimitDelay time.Duration
	lastRequest    time.Time
	mu             sync.Mutex

	// In-memory show list cache
	showCache     []models.AFLShow
	showCacheTime time.Time
	showCacheTTL  time.Duration
}

// New creates a new AnimeFillerList scraper.
func New(requestTimeout, rateLimitDelay, cacheTTL time.Duration) *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: requestTimeout,
		},
		rateLimitDelay: rateLimitDelay,
		showCacheTTL:   cacheTTL,
	}
}

// rateLimit ensures we don't hammer the site.
func (s *Scraper) rateLimit() {
	s.mu.Lock()
	defer s.mu.Unlock()

	since := time.Since(s.lastRequest)
	if since < s.rateLimitDelay {
		time.Sleep(s.rateLimitDelay - since)
	}
	s.lastRequest = time.Now()
}

// fetch performs an HTTP GET with polite headers and rate limiting.
func (s *Scraper) fetch(ctx context.Context, url string) (*goquery.Document, error) {
	s.rateLimit()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML from %s: %w", url, err)
	}

	return doc, nil
}

// GetAllShows fetches the complete list of shows from AnimeFillerList.
// Results are cached in memory for the configured TTL.
func (s *Scraper) GetAllShows(ctx context.Context) ([]models.AFLShow, error) {
	// Check cache
	if s.showCache != nil && time.Since(s.showCacheTime) < s.showCacheTTL {
		return s.showCache, nil
	}

	log.Info().Msg("fetching show list from AnimeFillerList")

	doc, err := s.fetch(ctx, baseURL+"/shows")
	if err != nil {
		return nil, fmt.Errorf("fetching show list: %w", err)
	}

	var shows []models.AFLShow

	// Parse: #ShowList .Group ul li a
	doc.Find("#ShowList .Group ul li a").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}

		title := strings.TrimSpace(sel.Text())
		slug := extractSlug(href)

		if title != "" && slug != "" {
			shows = append(shows, models.AFLShow{
				Title: title,
				Slug:  slug,
				URL:   baseURL + href,
			})
		}
	})

	log.Info().Int("count", len(shows)).Msg("fetched show list")

	// Update cache
	s.showCache = shows
	s.showCacheTime = time.Now()

	return shows, nil
}

// SearchShows searches the show list by title (case-insensitive substring match).
func (s *Scraper) SearchShows(ctx context.Context, query string) ([]models.AFLShow, error) {
	allShows, err := s.GetAllShows(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []models.AFLShow

	for _, show := range allShows {
		if strings.Contains(strings.ToLower(show.Title), query) {
			results = append(results, show)
		}
	}

	return results, nil
}

// GetShowEpisodes fetches all episodes for a show by its slug.
// Returns the show title and a slice of episodes with their filler classification.
func (s *Scraper) GetShowEpisodes(ctx context.Context, slug string) (string, []models.AFLEpisode, error) {
	url := fmt.Sprintf("%s/shows/%s", baseURL, slug)

	log.Info().Str("slug", slug).Str("url", url).Msg("fetching episode data")

	doc, err := s.fetch(ctx, url)
	if err != nil {
		return "", nil, fmt.Errorf("fetching episodes for %s: %w", slug, err)
	}

	// Extract show title from meta tag: <span property="dc:title" content="...">
	showTitle := ""
	doc.Find("span[property='dc:title']").Each(func(i int, sel *goquery.Selection) {
		if content, exists := sel.Attr("content"); exists {
			showTitle = strings.TrimSpace(content)
		}
	})
	if showTitle == "" {
		// Fallback: use the h1 text
		showTitle = strings.TrimSpace(doc.Find("h1").First().Text())
		showTitle = strings.TrimSuffix(showTitle, " Filler List")
	}

	// Parse episode table: table.EpisodeList tbody tr
	var episodes []models.AFLEpisode

	doc.Find("table.EpisodeList tbody tr").Each(func(i int, sel *goquery.Selection) {
		// Episode number
		numStr := strings.TrimSpace(sel.Find("td.Number").Text())
		num, err := strconv.Atoi(numStr)
		if err != nil {
			// Skip rows without valid episode numbers (e.g., header rows)
			return
		}

		// Episode title
		title := strings.TrimSpace(sel.Find("td.Title a").Text())
		if title == "" {
			title = strings.TrimSpace(sel.Find("td.Title").Text())
		}

		// Episode type from <td class="Type"><span>text</span></td>
		typeStr := strings.TrimSpace(sel.Find("td.Type span").Text())
		epType := parseEpisodeType(typeStr)

		episodes = append(episodes, models.AFLEpisode{
			Number: num,
			Title:  title,
			Type:   epType,
		})
	})

	log.Info().
		Str("show", showTitle).
		Int("episodes", len(episodes)).
		Msg("parsed episode data")

	return showTitle, episodes, nil
}

// parseEpisodeType converts the AFL type string to our EpisodeType.
func parseEpisodeType(typeStr string) models.EpisodeType {
	switch strings.ToLower(strings.TrimSpace(typeStr)) {
	case "manga canon":
		return models.EpisodeCanon
	case "anime canon":
		return models.EpisodeCanon // Treat anime canon as canon (anime-original but plot-relevant)
	case "mixed canon/filler":
		return models.EpisodeMixedCanon
	case "filler":
		return models.EpisodeFiller
	default:
		log.Warn().Str("type", typeStr).Msg("unknown episode type")
		return models.EpisodeUnknown
	}
}

// extractSlug extracts the show slug from an href like "/shows/naruto-shippuden".
func extractSlug(href string) string {
	href = strings.TrimPrefix(href, "/shows/")
	href = strings.TrimSuffix(href, "/")
	return href
}
'@ | Set-Content -Path "internal\scraper\scraper.go" -Encoding UTF8

# --- internal/sonarr/sonarr.go ---
@'
package sonarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/models"
	"github.com/rs/zerolog/log"
)

// Client communicates with the Sonarr API v3.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Sonarr API client.
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// doRequest performs an authenticated HTTP request to the Sonarr API.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v3%s", c.baseURL, path)

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode, string(errBody))
	}

	return resp, nil
}

// decodeResponse reads and decodes a JSON response body.
func decodeResponse[T any](resp *http.Response) (T, error) {
	var result T
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("decoding response: %w", err)
	}
	return result, nil
}

// --- Connection Test ---

// SystemStatus represents Sonarr's /system/status response.
type SystemStatus struct {
	Version   string `json:"version"`
	AppName   string `json:"appName"`
	StartTime string `json:"startTime"`
}

// TestConnection checks if the Sonarr instance is reachable and the API key is valid.
func (c *Client) TestConnection(ctx context.Context) (*SystemStatus, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/system/status", nil)
	if err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}
	return decodeResponse[*SystemStatus](resp)
}

// --- Series ---

// GetAllSeries returns all series from Sonarr.
func (c *Client) GetAllSeries(ctx context.Context) ([]models.SonarrSeries, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/series", nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrSeries](resp)
}

// GetAnimeSeries returns only anime-type series from Sonarr.
func (c *Client) GetAnimeSeries(ctx context.Context) ([]models.SonarrSeries, error) {
	allSeries, err := c.GetAllSeries(ctx)
	if err != nil {
		return nil, err
	}

	var anime []models.SonarrSeries
	for _, s := range allSeries {
		if s.SeriesType == "anime" {
			anime = append(anime, s)
		}
	}

	log.Debug().Int("total", len(allSeries)).Int("anime", len(anime)).Msg("filtered anime series")
	return anime, nil
}

// GetSeries returns a single series by ID.
func (c *Client) GetSeries(ctx context.Context, seriesID int) (*models.SonarrSeries, error) {
	path := fmt.Sprintf("/series/%d", seriesID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[*models.SonarrSeries](resp)
}

// UpdateSeries updates a series (used for applying tags).
func (c *Client) UpdateSeries(ctx context.Context, series *models.SonarrSeries) error {
	path := fmt.Sprintf("/series/%d", series.ID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, series)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// --- Episodes ---

// GetEpisodes returns all episodes for a series.
func (c *Client) GetEpisodes(ctx context.Context, seriesID int) ([]models.SonarrEpisode, error) {
	path := fmt.Sprintf("/episode?seriesId=%d", seriesID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrEpisode](resp)
}

// UpdateEpisode updates an episode (primarily to toggle monitored status).
func (c *Client) UpdateEpisode(ctx context.Context, episode *models.SonarrEpisode) error {
	path := fmt.Sprintf("/episode/%d", episode.ID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, episode)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// SetEpisodeMonitored sets the monitored status for an episode.
func (c *Client) SetEpisodeMonitored(ctx context.Context, episodeID int, monitored bool) error {
	// Sonarr v3 supports bulk monitoring via PUT /episode/monitor
	payload := map[string]interface{}{
		"episodeIds": []int{episodeID},
		"monitored":  monitored,
	}
	resp, err := c.doRequest(ctx, http.MethodPut, "/episode/monitor", payload)
	if err != nil {
		return fmt.Errorf("setting episode %d monitored=%v: %w", episodeID, monitored, err)
	}
	resp.Body.Close()
	return nil
}

// BulkSetEpisodesMonitored sets the monitored status for multiple episodes at once.
func (c *Client) BulkSetEpisodesMonitored(ctx context.Context, episodeIDs []int, monitored bool) error {
	if len(episodeIDs) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"episodeIds": episodeIDs,
		"monitored":  monitored,
	}
	resp, err := c.doRequest(ctx, http.MethodPut, "/episode/monitor", payload)
	if err != nil {
		return fmt.Errorf("bulk setting %d episodes monitored=%v: %w", len(episodeIDs), monitored, err)
	}
	resp.Body.Close()

	log.Info().
		Int("count", len(episodeIDs)).
		Bool("monitored", monitored).
		Msg("bulk updated episode monitoring")

	return nil
}

// --- Tags ---

// GetTags returns all tags from Sonarr.
func (c *Client) GetTags(ctx context.Context) ([]models.SonarrTag, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/tag", nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrTag](resp)
}

// CreateTag creates a new tag and returns it.
func (c *Client) CreateTag(ctx context.Context, label string) (*models.SonarrTag, error) {
	payload := map[string]string{"label": label}
	resp, err := c.doRequest(ctx, http.MethodPost, "/tag", payload)
	if err != nil {
		return nil, fmt.Errorf("creating tag %q: %w", label, err)
	}
	return decodeResponse[*models.SonarrTag](resp)
}

// EnsureTag finds or creates a tag by label, returning its ID.
func (c *Client) EnsureTag(ctx context.Context, label string) (int, error) {
	tags, err := c.GetTags(ctx)
	if err != nil {
		return 0, err
	}

	for _, t := range tags {
		if t.Label == label {
			return t.ID, nil
		}
	}

	// Tag doesn't exist, create it
	newTag, err := c.CreateTag(ctx, label)
	if err != nil {
		return 0, err
	}

	log.Info().Str("label", label).Int("id", newTag.ID).Msg("created new Sonarr tag")
	return newTag.ID, nil
}

// AddTagToSeries adds a tag to a series if it doesn't already have it.
func (c *Client) AddTagToSeries(ctx context.Context, series *models.SonarrSeries, tagID int) error {
	// Check if tag already applied
	for _, t := range series.Tags {
		if t == tagID {
			return nil // Already tagged
		}
	}

	series.Tags = append(series.Tags, tagID)
	return c.UpdateSeries(ctx, series)
}

// RemoveTagFromSeries removes a tag from a series.
func (c *Client) RemoveTagFromSeries(ctx context.Context, series *models.SonarrSeries, tagID int) error {
	newTags := make([]int, 0, len(series.Tags))
	for _, t := range series.Tags {
		if t != tagID {
			newTags = append(newTags, t)
		}
	}

	if len(newTags) == len(series.Tags) {
		return nil // Tag wasn't present
	}

	series.Tags = newTags
	return c.UpdateSeries(ctx, series)
}
'@ | Set-Content -Path "internal\sonarr\sonarr.go" -Encoding UTF8

# --- internal/sync/sync.go ---
@'
package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/config"
	"github.com/animefillerguard/animefillerguard/internal/database"
	"github.com/animefillerguard/animefillerguard/internal/matcher"
	"github.com/animefillerguard/animefillerguard/internal/models"
	"github.com/animefillerguard/animefillerguard/internal/notifier"
	"github.com/animefillerguard/animefillerguard/internal/scraper"
	"github.com/animefillerguard/animefillerguard/internal/sonarr"
	"github.com/rs/zerolog/log"
)

// Engine orchestrates the sync between AnimeFillerList and Sonarr.
type Engine struct {
	cfg     *config.Config
	db      *database.DB
	sonarr  *sonarr.Client
	scraper *scraper.Scraper
	matcher *matcher.Matcher

	notifier *notifier.Notifier

	// State
	running bool
}

// New creates a new sync engine.
func New(cfg *config.Config, db *database.DB) *Engine {
	return &Engine{
		cfg:     cfg,
		db:      db,
		sonarr:  sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, cfg.RequestTimeout),
		scraper: scraper.New(cfg.RequestTimeout, cfg.RateLimitDelay, cfg.CacheTTL),
		matcher:  matcher.New(0.7),
		notifier: notifier.New(cfg.NtfyServerURL, cfg.NtfyTopic, cfg.NtfyToken),
	}
}

// SonarrClient returns the underlying Sonarr client (for direct API calls from handlers).
func (e *Engine) SonarrClient() *sonarr.Client {
	return e.sonarr
}

// ScraperClient returns the underlying scraper (for AFL search from handlers).
func (e *Engine) ScraperClient() *scraper.Scraper {
	return e.scraper
}

// MatcherClient returns the underlying matcher (for manual matching from handlers).
func (e *Engine) MatcherClient() *matcher.Matcher {
	return e.matcher
}

// IsRunning returns whether a sync is currently in progress.
func (e *Engine) IsRunning() bool {
	return e.running
}

// RunSync executes a full sync cycle.
func (e *Engine) RunSync(ctx context.Context, trigger string) error {
	if e.running {
		return fmt.Errorf("sync already in progress")
	}
	if e.cfg.SonarrAPIKey == "" {
		log.Warn().Msg("skipping sync: no Sonarr API key configured")
		return fmt.Errorf("sonarr API key not configured")
	}

	e.running = true
	defer func() { e.running = false }()

	log.Info().Str("trigger", trigger).Msg("starting sync cycle")
	startTime := time.Now()

	// Create sync log entry
	logID, err := e.db.CreateSyncLog(ctx, trigger)
	if err != nil {
		log.Error().Err(err).Msg("failed to create sync log")
	}

	showsProcessed := 0
	episodesUpdated := 0
	var syncErrors []string

	// Step 1: Get all anime series from Sonarr
	animeSeries, err := e.sonarr.GetAnimeSeries(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch anime series from Sonarr: %v", err)
		log.Error().Err(err).Msg(errMsg)
		e.completeSyncLog(ctx, logID, "failed", 0, 0, errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Info().Int("anime_count", len(animeSeries)).Msg("fetched anime series from Sonarr")

	// Step 2: Get AFL show list for matching
	aflShows, err := e.scraper.GetAllShows(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch AFL show list: %v", err)
		log.Error().Err(err).Msg(errMsg)
		e.completeSyncLog(ctx, logID, "failed", 0, 0, errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Info().Int("afl_shows", len(aflShows)).Msg("fetched AFL show list")

	// Ensure filler tag exists if tagging is enabled
	var fillerTagID int
	if e.cfg.EnableTagging {
		fillerTagID, err = e.sonarr.EnsureTag(ctx, e.cfg.FillerTagName)
		if err != nil {
			log.Warn().Err(err).Msg("failed to ensure filler tag, continuing without tagging")
		}
	}

	// Step 3: Process each anime series
	for _, series := range animeSeries {
		updated, err := e.processSeries(ctx, series, aflShows, fillerTagID)
		if err != nil {
			errMsg := fmt.Sprintf("error processing %q: %v", series.Title, err)
			log.Error().Err(err).Str("series", series.Title).Msg("failed to process series")
			syncErrors = append(syncErrors, errMsg)
			continue
		}
		showsProcessed++
		episodesUpdated += updated
	}

	// Determine final status
	status := "success"
	if len(syncErrors) > 0 && showsProcessed == 0 {
		status = "failed"
	} else if len(syncErrors) > 0 {
		status = "partial"
	}

	errorsStr := strings.Join(syncErrors, "; ")
	e.completeSyncLog(ctx, logID, status, showsProcessed, episodesUpdated, errorsStr)

	log.Info().
		Str("status", status).
		Int("shows", showsProcessed).
		Int("episodes_updated", episodesUpdated).
		Int("errors", len(syncErrors)).
		Msg("sync cycle complete")

	// Send ntfy notification if configured
	if e.notifier != nil && e.notifier.Enabled() {
		result := notifier.SyncResult{
			Status:          status,
			ShowsProcessed:  showsProcessed,
			EpisodesUpdated: episodesUpdated,
			Errors:          syncErrors,
			Duration:        time.Since(startTime),
			Trigger:         trigger,
		}
		if err := e.notifier.SendSyncResult(ctx, result); err != nil {
			log.Warn().Err(err).Msg("failed to send ntfy notification")
		}
	}

	return nil
}

// processSeries handles a single anime series: match, scrape, and update episodes.
func (e *Engine) processSeries(ctx context.Context, series models.SonarrSeries, aflShows []models.AFLShow, fillerTagID int) (int, error) {
	// Check for existing mapping
	mapping, err := e.db.GetMappingBySonarrID(ctx, series.ID)
	if err != nil {
		return 0, fmt.Errorf("checking mapping: %w", err)
	}

	// If no mapping exists, try auto-matching
	if mapping == nil {
		mapping, err = e.autoMatch(ctx, series, aflShows)
		if err != nil || mapping == nil {
			// No match found -- skip this series (user can map manually)
			log.Debug().Str("series", series.Title).Msg("no AFL match found, skipping")
			return 0, nil
		}
	}

	// Skip if mapping is disabled
	if !mapping.Enabled {
		log.Debug().Str("series", series.Title).Msg("mapping disabled, skipping")
		return 0, nil
	}

	// Fetch episode filler data from AFL
	_, aflEpisodes, err := e.scraper.GetShowEpisodes(ctx, mapping.AFLSlug)
	if err != nil {
		return 0, fmt.Errorf("fetching AFL episodes: %w", err)
	}

	// Store AFL episode data in database
	var dbEpisodes []models.EpisodeInfo
	for _, ep := range aflEpisodes {
		dbEpisodes = append(dbEpisodes, models.EpisodeInfo{
			MappingID:  mapping.ID,
			EpisodeNum: ep.Number,
			Title:      ep.Title,
			Type:       ep.Type,
		})
	}
	if err := e.db.BulkUpsertEpisodes(ctx, dbEpisodes); err != nil {
		log.Warn().Err(err).Str("series", series.Title).Msg("failed to cache episode data")
	}

	// Build AFL episode lookup map: episode number -> type
	aflMap := make(map[int]models.EpisodeType)
	for _, ep := range aflEpisodes {
		aflMap[ep.Number] = ep.Type
	}

	// Fetch Sonarr episodes
	sonarrEpisodes, err := e.sonarr.GetEpisodes(ctx, series.ID)
	if err != nil {
		return 0, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}

	// Determine which episodes to unmonitor/monitor
	var toUnmonitor []int
	var toMonitor []int

	for _, ep := range sonarrEpisodes {
		// Skip specials (season 0)
		if ep.SeasonNumber == 0 {
			continue
		}

		// Map to AFL episode number
		aflNum := matcher.MapEpisodeNumber(ep, mapping.EpisodeOffset)

		epType, found := aflMap[aflNum]
		if !found {
			// No AFL data for this episode -- leave as-is
			continue
		}

		// Determine desired monitored state based on episode type and user preferences
		shouldMonitor := e.shouldMonitor(epType, mapping)

		if shouldMonitor && !ep.Monitored {
			toMonitor = append(toMonitor, ep.ID)
		} else if !shouldMonitor && ep.Monitored {
			toUnmonitor = append(toUnmonitor, ep.ID)
		}
	}

	// Apply changes in bulk
	updated := 0

	if len(toUnmonitor) > 0 {
		if err := e.sonarr.BulkSetEpisodesMonitored(ctx, toUnmonitor, false); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("failed to unmonitor episodes")
		} else {
			updated += len(toUnmonitor)
			log.Info().Str("series", series.Title).Int("count", len(toUnmonitor)).Msg("unmonitored filler episodes")
		}
	}

	if len(toMonitor) > 0 {
		if err := e.sonarr.BulkSetEpisodesMonitored(ctx, toMonitor, true); err != nil {
			log.Error().Err(err).Str("series", series.Title).Int("count", len(toMonitor)).Msg("failed to monitor episodes")
		} else {
			updated += len(toMonitor)
			log.Info().Str("series", series.Title).Int("count", len(toMonitor)).Msg("re-monitored canon episodes")
		}
	}

	// Apply filler tag to series if enabled
	if e.cfg.EnableTagging && fillerTagID > 0 && len(toUnmonitor) > 0 {
		if err := e.sonarr.AddTagToSeries(ctx, &series, fillerTagID); err != nil {
			log.Warn().Err(err).Str("series", series.Title).Msg("failed to add filler tag")
		}
	}

	return updated, nil
}

// autoMatch attempts to automatically match a Sonarr series to an AFL show.
func (e *Engine) autoMatch(ctx context.Context, series models.SonarrSeries, aflShows []models.AFLShow) (*models.ShowMapping, error) {
	bestMatch := e.matcher.BestMatch(ctx, series, aflShows)
	if bestMatch == nil {
		return nil, nil
	}

	log.Info().
		Str("sonarr", series.Title).
		Str("afl", bestMatch.AFLShow.Title).
		Float64("confidence", bestMatch.Confidence).
		Str("method", bestMatch.Method).
		Msg("auto-matched series")

	// Create mapping
	mapping := &models.ShowMapping{
		SonarrSeriesID:  series.ID,
		SonarrTitle:     series.Title,
		AFLSlug:         bestMatch.AFLShow.Slug,
		AFLTitle:        bestMatch.AFLShow.Title,
		AutoMatched:     true,
		MatchConfidence: bestMatch.Confidence,
		FillerAction:    models.FillerAction(e.cfg.DefaultFillerAction),
		MixedAction:     models.FillerAction(e.cfg.DefaultMixedCanonAction),
		Enabled:         true,
		EpisodeOffset:   0,
		CreatedAt:       time.Now(),
	}

	if err := e.db.UpsertMapping(ctx, mapping); err != nil {
		return nil, fmt.Errorf("saving auto-match: %w", err)
	}

	return mapping, nil
}

// shouldMonitor determines if an episode should be monitored based on its type and user rules.
func (e *Engine) shouldMonitor(epType models.EpisodeType, mapping *models.ShowMapping) bool {
	switch epType {
	case models.EpisodeCanon:
		// Canon episodes are always monitored
		return true

	case models.EpisodeFiller:
		switch mapping.FillerAction {
		case models.ActionUnmonitor, models.ActionSkip:
			return false
		case models.ActionMonitor:
			return true
		default:
			return false
		}

	case models.EpisodeMixedCanon:
		switch mapping.MixedAction {
		case models.ActionUnmonitor, models.ActionSkip:
			return false
		case models.ActionMonitor:
			return true
		default:
			return true // Default: keep mixed canon
		}

	case models.EpisodeUnknown:
		// Unknown episodes are monitored by default (safe fallback)
		return true

	default:
		return true
	}
}

// completeSyncLog helper to update sync log with final results.
func (e *Engine) completeSyncLog(ctx context.Context, logID int64, status string, shows, episodes int, errors string) {
	if logID <= 0 {
		return
	}
	if err := e.db.CompleteSyncLog(ctx, logID, status, shows, episodes, errors); err != nil {
		log.Error().Err(err).Msg("failed to complete sync log")
	}
}
'@ | Set-Content -Path "internal\sync\sync.go" -Encoding UTF8

# --- internal/web/embed.go ---
@'
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
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
'@ | Set-Content -Path "internal\web\embed.go" -Encoding UTF8

# --- web/index.html ---
@'
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>AnimeFillerGuard</title>
    <style>
      html, body {
        margin: 0;
        padding: 0;
        background: #0f1117;
        color: #e2e8f0;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      }
    </style>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.js"></script>
  </body>
</html>
'@ | Set-Content -Path "web\index.html" -Encoding UTF8

# --- web/package.json ---
@'
{
  "name": "animefillerguard-web",
  "private": true,
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^3.1.0",
    "svelte": "^4.2.12",
    "vite": "^5.2.0"
  },
  "dependencies": {
    "svelte-spa-router": "^4.0.1"
  }
}
'@ | Set-Content -Path "web\package.json" -Encoding UTF8

# --- web/src/App.svelte ---
@'
<script>
  import Router from 'svelte-spa-router';
  import Navbar from './components/Navbar.svelte';
  import Toast from './components/Toast.svelte';

  import Dashboard from './pages/Dashboard.svelte';
  import Shows from './pages/Shows.svelte';
  import ShowDetail from './pages/ShowDetail.svelte';
  import Settings from './pages/Settings.svelte';
  import Logs from './pages/Logs.svelte';

  const routes = {
    '/': Dashboard,
    '/shows': Shows,
    '/shows/:id': ShowDetail,
    '/settings': Settings,
    '/logs': Logs,
  };
</script>

<div class="app-layout">
  <Navbar />
  <main class="main-content">
    <Router {routes} />
  </main>
  <Toast />
</div>
'@ | Set-Content -Path "web\src\App.svelte" -Encoding UTF8

# --- web/src/app.css ---
@'
/* ============================================================
   AnimeFillerGuard - Global Styles
   Dark theme matching the *arr ecosystem (Sonarr/Radarr)
   ============================================================ */

/* --- CSS Variables --- */
:root {
  /* Core palette */
  --bg-primary: #0f1117;
  --bg-secondary: #1a1d29;
  --bg-tertiary: #232738;
  --bg-hover: #2a2e3f;
  --bg-active: #323750;

  /* Text */
  --text-primary: #e2e8f0;
  --text-secondary: #94a3b8;
  --text-muted: #64748b;
  --text-inverse: #0f1117;

  /* Accent */
  --accent: #3b82f6;
  --accent-hover: #2563eb;
  --accent-muted: rgba(59, 130, 246, 0.15);

  /* Episode type colors */
  --canon: #22c55e;
  --canon-bg: rgba(34, 197, 94, 0.12);
  --mixed: #eab308;
  --mixed-bg: rgba(234, 179, 8, 0.12);
  --anime-canon: #f97316;
  --anime-canon-bg: rgba(249, 115, 22, 0.12);
  --filler: #ef4444;
  --filler-bg: rgba(239, 68, 68, 0.12);

  /* Status */
  --success: #22c55e;
  --success-bg: rgba(34, 197, 94, 0.12);
  --warning: #eab308;
  --warning-bg: rgba(234, 179, 8, 0.12);
  --danger: #ef4444;
  --danger-bg: rgba(239, 68, 68, 0.12);
  --info: #3b82f6;
  --info-bg: rgba(59, 130, 246, 0.12);

  /* Borders */
  --border: #2d3148;
  --border-light: #3a3f57;

  /* Shadows */
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.3);
  --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.4);
  --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.5);

  /* Layout */
  --sidebar-width: 220px;
  --header-height: 0px;
  --radius: 6px;
  --radius-lg: 10px;

  /* Transitions */
  --transition: 150ms ease;
}

/* --- Reset & Base --- */
*, *::before, *::after {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

html, body {
  height: 100%;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  font-size: 14px;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
}

#app {
  height: 100%;
}

a {
  color: var(--accent);
  text-decoration: none;
  transition: color var(--transition);
}
a:hover {
  color: var(--accent-hover);
}

/* --- Layout: Sidebar + Main --- */
.app-layout {
  display: flex;
  min-height: 100vh;
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 100;
  overflow-y: auto;
}

.sidebar-brand {
  padding: 20px 16px 16px;
  border-bottom: 1px solid var(--border);
}

.sidebar-brand h1 {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.3px;
}

.sidebar-brand .brand-icon {
  font-size: 22px;
  margin-right: 8px;
}

.sidebar-brand .brand-sub {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.sidebar-nav {
  padding: 12px 8px;
  flex: 1;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: var(--radius);
  color: var(--text-secondary);
  font-size: 13.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition);
  margin-bottom: 2px;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--accent-muted);
  color: var(--accent);
}

.nav-item .nav-icon {
  width: 18px;
  text-align: center;
  font-size: 15px;
  opacity: 0.8;
}

.sidebar-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  font-size: 11px;
  color: var(--text-muted);
}

.main-content {
  flex: 1;
  margin-left: var(--sidebar-width);
  padding: 24px 28px;
  min-height: 100vh;
  overflow-x: hidden;
}

/* --- Page Header --- */
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-header h2 {
  font-size: 22px;
  font-weight: 600;
  color: var(--text-primary);
}

.page-header p {
  color: var(--text-secondary);
  font-size: 13px;
  margin-top: 4px;
}

/* --- Cards --- */
.card {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 20px;
  transition: border-color var(--transition);
}

.card:hover {
  border-color: var(--border-light);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.card-header h3 {
  font-size: 15px;
  font-weight: 600;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 12px;
}

/* --- Stats Grid --- */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-card .stat-label {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 500;
}

.stat-card .stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.2;
}

.stat-card .stat-sub {
  font-size: 12px;
  color: var(--text-secondary);
}

.stat-card.accent .stat-value {
  color: var(--accent);
}

.stat-card.success .stat-value {
  color: var(--success);
}

.stat-card.danger .stat-value {
  color: var(--danger);
}

.stat-card.warning .stat-value {
  color: var(--warning);
}

/* --- Tables --- */
.table-container {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

thead {
  background: var(--bg-tertiary);
}

th {
  padding: 10px 14px;
  text-align: left;
  font-weight: 600;
  font-size: 11.5px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}

td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  color: var(--text-primary);
  vertical-align: middle;
}

tr:last-child td {
  border-bottom: none;
}

tbody tr {
  transition: background var(--transition);
}

tbody tr:hover {
  background: var(--bg-hover);
}

/* Episode type row highlights */
tr.ep-canon { background: var(--canon-bg); }
tr.ep-canon:hover { background: rgba(34, 197, 94, 0.18); }
tr.ep-mixed { background: var(--mixed-bg); }
tr.ep-mixed:hover { background: rgba(234, 179, 8, 0.18); }
tr.ep-anime-canon { background: var(--anime-canon-bg); }
tr.ep-anime-canon:hover { background: rgba(249, 115, 22, 0.18); }
tr.ep-filler { background: var(--filler-bg); }
tr.ep-filler:hover { background: rgba(239, 68, 68, 0.18); }

/* --- Buttons --- */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: var(--radius);
  border: 1px solid transparent;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition);
  white-space: nowrap;
  font-family: inherit;
  line-height: 1.4;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}
.btn-primary:hover:not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}

.btn-secondary {
  background: var(--bg-tertiary);
  color: var(--text-primary);
  border-color: var(--border);
}
.btn-secondary:hover:not(:disabled) {
  background: var(--bg-hover);
  border-color: var(--border-light);
}

.btn-danger {
  background: var(--danger-bg);
  color: var(--danger);
  border-color: rgba(239, 68, 68, 0.3);
}
.btn-danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.2);
  border-color: var(--danger);
}

.btn-success {
  background: var(--success-bg);
  color: var(--success);
  border-color: rgba(34, 197, 94, 0.3);
}
.btn-success:hover:not(:disabled) {
  background: rgba(34, 197, 94, 0.2);
  border-color: var(--success);
}

.btn-sm {
  padding: 5px 10px;
  font-size: 12px;
}

.btn-lg {
  padding: 10px 22px;
  font-size: 14px;
}

.btn-icon {
  padding: 6px 8px;
  min-width: 32px;
  justify-content: center;
}

.btn-group {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* --- Forms --- */
.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.form-input,
.form-select {
  width: 100%;
  padding: 9px 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  font-size: 13.5px;
  font-family: inherit;
  transition: border-color var(--transition);
  outline: none;
}

.form-input:focus,
.form-select:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-muted);
}

.form-input::placeholder {
  color: var(--text-muted);
}

.form-select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%2394a3b8' d='M2 4l4 4 4-4'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 12px center;
  padding-right: 32px;
  cursor: pointer;
}

.form-hint {
  font-size: 11.5px;
  color: var(--text-muted);
  margin-top: 4px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.form-inline {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Toggle / Switch */
.toggle {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
  flex-shrink: 0;
}

.toggle input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  inset: 0;
  background: var(--bg-tertiary);
  border: 1px solid var(--border);
  border-radius: 11px;
  cursor: pointer;
  transition: all var(--transition);
}

.toggle-slider::before {
  content: '';
  position: absolute;
  width: 16px;
  height: 16px;
  left: 2px;
  bottom: 2px;
  background: var(--text-secondary);
  border-radius: 50%;
  transition: all var(--transition);
}

.toggle input:checked + .toggle-slider {
  background: var(--accent);
  border-color: var(--accent);
}

.toggle input:checked + .toggle-slider::before {
  transform: translateX(18px);
  background: white;
}

/* --- Badges --- */
.badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  white-space: nowrap;
}

.badge-canon {
  background: var(--canon-bg);
  color: var(--canon);
  border: 1px solid rgba(34, 197, 94, 0.25);
}

.badge-mixed {
  background: var(--mixed-bg);
  color: var(--mixed);
  border: 1px solid rgba(234, 179, 8, 0.25);
}

.badge-anime-canon {
  background: var(--anime-canon-bg);
  color: var(--anime-canon);
  border: 1px solid rgba(249, 115, 22, 0.25);
}

.badge-filler {
  background: var(--filler-bg);
  color: var(--filler);
  border: 1px solid rgba(239, 68, 68, 0.25);
}

.badge-unknown {
  background: var(--bg-tertiary);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.badge-success {
  background: var(--success-bg);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.25);
}

.badge-warning {
  background: var(--warning-bg);
  color: var(--warning);
  border: 1px solid rgba(234, 179, 8, 0.25);
}

.badge-danger {
  background: var(--danger-bg);
  color: var(--danger);
  border: 1px solid rgba(239, 68, 68, 0.25);
}

.badge-info {
  background: var(--info-bg);
  color: var(--info);
  border: 1px solid rgba(59, 130, 246, 0.25);
}

/* --- Status Dots --- */
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
}

.status-dot.online { background: var(--success); box-shadow: 0 0 6px var(--success); }
.status-dot.offline { background: var(--danger); box-shadow: 0 0 6px var(--danger); }
.status-dot.warning { background: var(--warning); box-shadow: 0 0 6px var(--warning); }

/* --- Modal --- */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  animation: fadeIn 150ms ease;
}

.modal {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 540px;
  max-height: 85vh;
  overflow-y: auto;
  box-shadow: var(--shadow-lg);
  animation: slideUp 200ms ease;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  font-size: 16px;
  font-weight: 600;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 20px;
  padding: 4px;
  line-height: 1;
  transition: color var(--transition);
}

.modal-close:hover {
  color: var(--text-primary);
}

.modal-body {
  padding: 20px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
}

/* --- Toast / Notifications --- */
.toast-container {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}

.toast {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 12px 16px;
  min-width: 280px;
  max-width: 420px;
  box-shadow: var(--shadow-lg);
  display: flex;
  align-items: center;
  gap: 10px;
  pointer-events: all;
  animation: slideInRight 250ms ease;
}

.toast-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.toast-message {
  flex: 1;
  font-size: 13px;
  color: var(--text-primary);
}

.toast-close {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 14px;
  padding: 2px;
}

.toast.toast-success { border-left: 3px solid var(--success); }
.toast.toast-error { border-left: 3px solid var(--danger); }
.toast.toast-warning { border-left: 3px solid var(--warning); }
.toast.toast-info { border-left: 3px solid var(--info); }

/* --- Spinner --- */
.spinner {
  display: inline-block;
  width: 20px;
  height: 20px;
  border: 2px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 600ms linear infinite;
}

.spinner-lg {
  width: 36px;
  height: 36px;
  border-width: 3px;
}

.spinner-center {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
}

/* --- Search Bar --- */
.search-bar {
  position: relative;
  max-width: 320px;
}

.search-bar .search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-muted);
  font-size: 14px;
  pointer-events: none;
}

.search-bar .form-input {
  padding-left: 36px;
}

/* --- Confidence Bar --- */
.confidence-bar {
  width: 60px;
  height: 6px;
  background: var(--bg-tertiary);
  border-radius: 3px;
  overflow: hidden;
  display: inline-block;
  vertical-align: middle;
  margin-right: 6px;
}

.confidence-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 300ms ease;
}

.confidence-fill.high { background: var(--success); }
.confidence-fill.medium { background: var(--warning); }
.confidence-fill.low { background: var(--danger); }

/* --- Empty State --- */
.empty-state {
  text-align: center;
  padding: 48px 20px;
  color: var(--text-muted);
}

.empty-state .empty-icon {
  font-size: 48px;
  margin-bottom: 16px;
  opacity: 0.4;
}

.empty-state h3 {
  font-size: 16px;
  color: var(--text-secondary);
  margin-bottom: 8px;
}

.empty-state p {
  font-size: 13px;
  max-width: 360px;
  margin: 0 auto 16px;
}

/* --- Grids --- */
.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.grid-3 {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 16px;
}

/* --- Section --- */
.section {
  margin-bottom: 28px;
}

.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 12px;
}

/* --- Show Detail Header --- */
.show-header {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  margin-bottom: 24px;
}

.show-info-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 20px;
}

.show-info-card h3 {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 12px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 6px 0;
  font-size: 13px;
}

.info-row .info-label {
  color: var(--text-secondary);
}

.info-row .info-value {
  color: var(--text-primary);
  font-weight: 500;
}

/* --- Episode Controls Bar --- */
.controls-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  margin-bottom: 16px;
  flex-wrap: wrap;
}

/* --- Expandable Row --- */
.expand-toggle {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  padding: 4px;
  transition: transform var(--transition);
}

.expand-toggle.expanded {
  transform: rotate(90deg);
}

.expanded-content {
  background: var(--bg-primary);
}

.expanded-content td {
  padding: 12px 14px 12px 42px;
  font-size: 12px;
  color: var(--text-secondary);
}

/* --- Animations --- */
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes slideUp {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes slideInRight {
  from { opacity: 0; transform: translateX(40px); }
  to { opacity: 1; transform: translateX(0); }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* --- Responsive --- */
@media (max-width: 768px) {
  .sidebar {
    width: 60px;
  }
  .sidebar-brand h1,
  .sidebar-brand .brand-sub,
  .nav-item span:not(.nav-icon),
  .sidebar-footer {
    display: none;
  }
  .nav-item {
    justify-content: center;
    padding: 12px;
  }
  .main-content {
    margin-left: 60px;
    padding: 16px;
  }
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .show-header,
  .grid-2,
  .form-row {
    grid-template-columns: 1fr;
  }
  .page-header {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (max-width: 480px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
  .controls-bar {
    flex-direction: column;
    align-items: stretch;
  }
}
'@ | Set-Content -Path "web\src\app.css" -Encoding UTF8

# --- web/src/components/Badge.svelte ---
@'
<script>
  export let type = 'unknown'; // canon, mixed_canon, anime_canon, filler, unknown
  export let label = '';

  const typeMap = {
    canon: { class: 'badge-canon', text: 'Canon' },
    mixed_canon: { class: 'badge-mixed', text: 'Mixed' },
    mixed: { class: 'badge-mixed', text: 'Mixed' },
    anime_canon: { class: 'badge-anime-canon', text: 'Anime Canon' },
    filler: { class: 'badge-filler', text: 'Filler' },
    unknown: { class: 'badge-unknown', text: 'Unknown' },
    success: { class: 'badge-success', text: 'Success' },
    warning: { class: 'badge-warning', text: 'Warning' },
    danger: { class: 'badge-danger', text: 'Danger' },
    error: { class: 'badge-danger', text: 'Error' },
    info: { class: 'badge-info', text: 'Info' },
    partial: { class: 'badge-warning', text: 'Partial' },
    failed: { class: 'badge-danger', text: 'Failed' },
    running: { class: 'badge-info', text: 'Running' },
  };

  $: info = typeMap[type] || typeMap.unknown;
  $: displayText = label || info.text;
</script>

<span class="badge {info.class}">{displayText}</span>
'@ | Set-Content -Path "web\src\components\Badge.svelte" -Encoding UTF8

# --- web/src/components/Modal.svelte ---
@'
<script>
  import { createEventDispatcher } from 'svelte';

  export let title = '';
  export let show = false;
  export let size = 'default'; // 'default', 'large'

  const dispatch = createEventDispatcher();

  function close() {
    dispatch('close');
  }

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) {
      close();
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      close();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if show}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click={handleBackdropClick}>
    <div class="modal" style={size === 'large' ? 'max-width:720px' : ''}>
      <div class="modal-header">
        <h3>{title}</h3>
        <button class="modal-close" on:click={close}>&times;</button>
      </div>
      <div class="modal-body">
        <slot />
      </div>
      {#if $$slots.footer}
        <div class="modal-footer">
          <slot name="footer" />
        </div>
      {/if}
    </div>
  </div>
{/if}
'@ | Set-Content -Path "web\src\components\Modal.svelte" -Encoding UTF8

# --- web/src/components/Navbar.svelte ---
@'
<script>
  import { location } from 'svelte-spa-router';
  import { syncStatus } from '../lib/stores.js';

  const navItems = [
    { path: '/', label: 'Dashboard', icon: '&#9632;' },
    { path: '/shows', label: 'Shows', icon: '&#9654;' },
    { path: '/settings', label: 'Settings', icon: '&#9881;' },
    { path: '/logs', label: 'Logs', icon: '&#9776;' },
  ];

  function isActive(itemPath, currentPath) {
    if (itemPath === '/') return currentPath === '/';
    return currentPath.startsWith(itemPath);
  }
</script>

<aside class="sidebar">
  <div class="sidebar-brand">
    <h1><span class="brand-icon">&#9878;</span>AFG</h1>
    <div class="brand-sub">AnimeFillerGuard</div>
  </div>

  <nav class="sidebar-nav">
    {#each navItems as item}
      <a
        href="#{item.path}"
        class="nav-item"
        class:active={isActive(item.path, $location)}
      >
        <span class="nav-icon">{@html item.icon}</span>
        <span>{item.label}</span>
      </a>
    {/each}
  </nav>

  <div class="sidebar-footer">
    <div style="display:flex;align-items:center;gap:6px;margin-bottom:4px">
      <span class="status-dot" class:online={$syncStatus.connected} class:offline={!$syncStatus.connected}></span>
      <span>{$syncStatus.connected ? 'Sonarr Connected' : 'Sonarr Offline'}</span>
    </div>
    {#if $syncStatus.syncing}
      <div style="color:var(--accent)">Syncing...</div>
    {/if}
  </div>
</aside>
'@ | Set-Content -Path "web\src\components\Navbar.svelte" -Encoding UTF8

# --- web/src/components/Spinner.svelte ---
@'
<script>
  export let size = 'default'; // 'default', 'lg'
  export let center = false;
  export let message = '';
</script>

{#if center}
  <div class="spinner-center">
    <div>
      <div class="spinner" class:spinner-lg={size === 'lg'}></div>
      {#if message}
        <p style="margin-top:12px;color:var(--text-secondary);font-size:13px;text-align:center">{message}</p>
      {/if}
    </div>
  </div>
{:else}
  <span class="spinner" class:spinner-lg={size === 'lg'}></span>
{/if}
'@ | Set-Content -Path "web\src\components\Spinner.svelte" -Encoding UTF8

# --- web/src/components/Toast.svelte ---
@'
<script>
  import { notifications, removeNotification } from '../lib/stores.js';

  const icons = {
    success: '&#10003;',
    error: '&#10007;',
    warning: '&#9888;',
    info: '&#8505;',
  };
</script>

<div class="toast-container">
  {#each $notifications as notif (notif.id)}
    <div class="toast toast-{notif.type}">
      <span class="toast-icon">{@html icons[notif.type] || icons.info}</span>
      <span class="toast-message">{notif.message}</span>
      <button class="toast-close" on:click={() => removeNotification(notif.id)}>&times;</button>
    </div>
  {/each}
</div>
'@ | Set-Content -Path "web\src\components\Toast.svelte" -Encoding UTF8

# --- web/src/lib/api.js ---
@'
const BASE = '/api';

async function request(method, path, body = null) {
  const opts = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body) opts.body = JSON.stringify(body);

  const res = await fetch(BASE + path, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

// Health & Status
export function getStatus() {
  return request('GET', '/health');
}

export function getVersion() {
  return request('GET', '/version');
}

export function getStats() {
  return request('GET', '/stats');
}

// Sonarr
export function testSonarrConnection(url, apiKey) {
  return request('POST', '/sonarr/test', { url, api_key: apiKey });
}

// Show Mappings
export function getShows() {
  return request('GET', '/mappings');
}

export function getShow(id) {
  return request('GET', `/mappings/${id}`);
}

export function createShow(data) {
  return request('POST', '/mappings', data);
}

export function updateShow(id, data) {
  return request('PUT', `/mappings/${id}`, data);
}

export function deleteShow(id) {
  return request('DELETE', `/mappings/${id}`);
}

// Auto-match / confirm
export function autoMatch(sonarrSeriesId) {
  return request('POST', '/mappings', { sonarr_series_id: sonarrSeriesId, auto_match: true });
}

export function confirmMatch(sonarrSeriesId, aflSlug) {
  return request('POST', '/mappings', { sonarr_series_id: sonarrSeriesId, afl_slug: aflSlug });
}

// AFL Search
export function searchAFL(query) {
  return request('GET', `/afl/search?q=${encodeURIComponent(query)}`);
}

// Episodes
export function getEpisodes(showId) {
  return request('GET', `/mappings/${showId}/episodes`);
}

// Sync
export function triggerSync() {
  return request('POST', '/sync');
}

export function getSyncLogs() {
  return request('GET', '/logs');
}

// Settings
export function getSettings() {
  return request('GET', '/settings');
}

export function updateSettings(data) {
  return request('PUT', '/settings', data);
}
'@ | Set-Content -Path "web\src\lib\api.js" -Encoding UTF8

# --- web/src/lib/stores.js ---
@'
import { writable } from 'svelte/store';

export const shows = writable([]);
export const syncStatus = writable({
  connected: false,
  aflReachable: false,
  lastSync: null,
  nextSync: null,
  syncing: false,
});
export const settings = writable({
  sonarr_url: '',
  sonarr_api_key: '',
  default_filler_action: 'unmonitor',
  default_mixed_action: 'monitor',
  sync_interval: '6h',
  enable_tagging: false,
  cache_ttl: '24h',
});
export const notifications = writable([]);

let notifId = 0;

export function addNotification(message, type = 'info', duration = 4000) {
  const id = ++notifId;
  notifications.update(n => [...n, { id, message, type, duration }]);
  if (duration > 0) {
    setTimeout(() => removeNotification(id), duration);
  }
}

export function removeNotification(id) {
  notifications.update(n => n.filter(x => x.id !== id));
}
'@ | Set-Content -Path "web\src\lib\stores.js" -Encoding UTF8

# --- web/src/main.js ---
@'
import App from './App.svelte';
import './app.css';

const app = new App({
  target: document.getElementById('app'),
});

export default app;
'@ | Set-Content -Path "web\src\main.js" -Encoding UTF8

# --- web/src/pages/Dashboard.svelte ---
@'
<script>
  import { onMount } from 'svelte';
  import { shows, syncStatus, addNotification } from '../lib/stores.js';
  import { getStatus, getStats, getShows, getSyncLogs, triggerSync, getVersion } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';

  let loading = true;
  let stats = { canon: 0, mixed: 0, filler: 0, total: 0 };
  let recentLogs = [];
  let version = '';
  let syncing = false;
  let sonarrConnected = false;
  let aflReachable = false;
  let mappedCount = 0;

  onMount(async () => {
    await loadDashboard();
  });

  async function loadDashboard() {
    loading = true;
    try {
      const [statusRes, statsRes, showsRes, logsRes, versionRes] = await Promise.allSettled([
        getStatus(),
        getStats(),
        getShows(),
        getSyncLogs(),
        getVersion(),
      ]);

      if (statusRes.status === 'fulfilled') {
        sonarrConnected = statusRes.value.status === 'ok';
        aflReachable = true;
        syncStatus.update(s => ({ ...s, connected: sonarrConnected, aflReachable: true }));
      }

      if (statsRes.status === 'fulfilled') {
        stats = statsRes.value;
      }

      if (showsRes.status === 'fulfilled') {
        const allShows = showsRes.value || [];
        shows.set(allShows);
        mappedCount = allShows.length;
      }

      if (logsRes.status === 'fulfilled') {
        recentLogs = (logsRes.value || []).slice(0, 5);
      }

      if (versionRes.status === 'fulfilled') {
        version = versionRes.value.version || '';
      }
    } catch (err) {
      addNotification('Failed to load dashboard data', 'error');
    }
    loading = false;
  }

  async function handleSync() {
    syncing = true;
    syncStatus.update(s => ({ ...s, syncing: true }));
    try {
      await triggerSync();
      addNotification('Sync triggered successfully', 'success');
      setTimeout(loadDashboard, 2000);
    } catch (err) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    syncing = false;
    syncStatus.update(s => ({ ...s, syncing: false }));
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return '-';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return ms + 'ms';
    return (ms / 1000).toFixed(1) + 's';
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }
</script>

{#if loading}
  <Spinner center size="lg" message="Loading dashboard..." />
{:else}
  <div class="page-header">
    <div>
      <h2>Dashboard</h2>
      <p>AnimeFillerGuard overview {version ? `v${version}` : ''}</p>
    </div>
    <button class="btn btn-primary" on:click={handleSync} disabled={syncing}>
      {#if syncing}
        <span class="spinner"></span> Syncing...
      {:else}
        &#8635; Sync Now
      {/if}
    </button>
  </div>

  <!-- Connection Status -->
  <div class="section">
    <div class="section-title">Connection Status</div>
    <div class="grid-2" style="max-width:500px">
      <div class="card" style="padding:14px 18px;display:flex;align-items:center;gap:10px">
        <span class="status-dot" class:online={sonarrConnected} class:offline={!sonarrConnected}></span>
        <div>
          <div style="font-size:13px;font-weight:600">{sonarrConnected ? 'Sonarr Connected' : 'Sonarr Offline'}</div>
          <div style="font-size:11px;color:var(--text-muted)">{sonarrConnected ? 'API responding' : 'Cannot reach API'}</div>
        </div>
      </div>
      <div class="card" style="padding:14px 18px;display:flex;align-items:center;gap:10px">
        <span class="status-dot" class:online={aflReachable} class:offline={!aflReachable}></span>
        <div>
          <div style="font-size:13px;font-weight:600">{aflReachable ? 'AFL Reachable' : 'AFL Unreachable'}</div>
          <div style="font-size:11px;color:var(--text-muted)">AnimeFillerList.com</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Stats Cards -->
  <div class="section">
    <div class="section-title">Overview</div>
    <div class="stats-grid">
      <div class="stat-card accent">
        <span class="stat-label">Mapped Shows</span>
        <span class="stat-value">{mappedCount}</span>
        <span class="stat-sub">Series tracked</span>
      </div>
      <div class="stat-card success">
        <span class="stat-label">Canon Episodes</span>
        <span class="stat-value">{stats.canon}</span>
        <span class="stat-sub">Monitored</span>
      </div>
      <div class="stat-card warning">
        <span class="stat-label">Mixed Episodes</span>
        <span class="stat-value">{stats.mixed}</span>
        <span class="stat-sub">Canon/Filler blend</span>
      </div>
      <div class="stat-card danger">
        <span class="stat-label">Filler Episodes</span>
        <span class="stat-value">{stats.filler}</span>
        <span class="stat-sub">Skipped / Unmonitored</span>
      </div>
    </div>
  </div>

  <!-- Recent Sync Logs -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Recent Syncs</h3>
        <a href="#/logs" class="btn btn-sm btn-secondary">View All</a>
      </div>
      {#if recentLogs.length === 0}
        <div class="empty-state" style="padding:24px">
          <p>No sync runs yet. Click "Sync Now" to start.</p>
        </div>
      {:else}
        <div class="table-container" style="border:none;border-radius:0">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Duration</th>
                <th>Shows</th>
                <th>Ep. Changed</th>
                <th>Trigger</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {#each recentLogs as log}
                <tr>
                  <td>{formatTime(log.started_at)}</td>
                  <td>{formatDuration(log.started_at, log.completed_at)}</td>
                  <td>{log.shows_processed}</td>
                  <td>{log.episodes_updated}</td>
                  <td style="text-transform:capitalize">{log.trigger || log.trigger_type || '-'}</td>
                  <td><Badge type={log.status} /></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>

  <!-- Quick Add -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Quick Add</h3>
        <a href="#/shows" class="btn btn-sm btn-secondary">Manage Shows</a>
      </div>
      <div style="padding:4px 0">
        {#if mappedCount === 0}
          <div class="empty-state" style="padding:20px">
            <div class="empty-icon">&#128253;</div>
            <h3>No Shows Mapped Yet</h3>
            <p>Head to the Shows page to map your Sonarr anime series to AnimeFillerList.</p>
            <a href="#/shows" class="btn btn-primary">Go to Shows</a>
          </div>
        {:else}
          <p style="font-size:13px;color:var(--text-secondary);padding:4px 0">
            {mappedCount} show{mappedCount === 1 ? '' : 's'} currently mapped and tracking filler data.
            Visit the Shows page to add more or manage existing mappings.
          </p>
        {/if}
      </div>
    </div>
  </div>
{/if}
'@ | Set-Content -Path "web\src\pages\Dashboard.svelte" -Encoding UTF8

# --- web/src/pages/Logs.svelte ---
@'
<script>
  import { onMount, onDestroy } from 'svelte';
  import { addNotification } from '../lib/stores.js';
  import { getSyncLogs } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';

  let loading = true;
  let logs = [];
  let expandedId = null;
  let autoRefresh = false;
  let refreshInterval = null;

  onMount(async () => {
    await loadLogs();
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  async function loadLogs() {
    loading = logs.length === 0;
    try {
      logs = await getSyncLogs() || [];
    } catch (err) {
      addNotification('Failed to load sync logs: ' + err.message, 'error');
    }
    loading = false;
  }

  function toggleAutoRefresh() {
    autoRefresh = !autoRefresh;
    if (autoRefresh) {
      refreshInterval = setInterval(loadLogs, 5000);
      addNotification('Auto-refresh enabled (5s)', 'info', 2000);
    } else {
      if (refreshInterval) clearInterval(refreshInterval);
      refreshInterval = null;
    }
  }

  function toggleExpand(id) {
    expandedId = expandedId === id ? null : id;
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return '-';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    return Math.floor(ms / 60000) + 'm ' + Math.round((ms % 60000) / 1000) + 's';
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }

  function statusType(status) {
    switch (status) {
      case 'success': return 'success';
      case 'partial': return 'warning';
      case 'failed': return 'failed';
      case 'running': return 'running';
      default: return 'unknown';
    }
  }
</script>

<div class="page-header">
  <div>
    <h2>Sync Logs</h2>
    <p>{logs.length} sync run{logs.length === 1 ? '' : 's'} recorded</p>
  </div>
  <div class="btn-group">
    <button
      class="btn btn-sm {autoRefresh ? 'btn-primary' : 'btn-secondary'}"
      on:click={toggleAutoRefresh}
    >
      {autoRefresh ? '&#9210; Auto-Refresh ON' : '&#9210; Auto-Refresh OFF'}
    </button>
    <button class="btn btn-sm btn-secondary" on:click={loadLogs}>
      &#8635; Refresh
    </button>
  </div>
</div>

{#if loading}
  <Spinner center size="lg" message="Loading logs..." />
{:else if logs.length === 0}
  <div class="card">
    <div class="empty-state">
      <div class="empty-icon">&#128220;</div>
      <h3>No Sync Logs</h3>
      <p>Sync logs will appear here after your first sync run. Trigger a sync from the Dashboard.</p>
      <a href="#/" class="btn btn-primary">Go to Dashboard</a>
    </div>
  </div>
{:else}
  <div class="table-container">
    <table>
      <thead>
        <tr>
          <th style="width:30px"></th>
          <th>Started</th>
          <th>Duration</th>
          <th>Shows Processed</th>
          <th>Episodes Changed</th>
          <th>Trigger</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {#each logs as log}
          <tr>
            <td>
              <button
                class="expand-toggle"
                class:expanded={expandedId === log.id}
                on:click={() => toggleExpand(log.id)}
              >
                &#9654;
              </button>
            </td>
            <td>{formatTime(log.started_at)}</td>
            <td>{formatDuration(log.started_at, log.completed_at)}</td>
            <td>{log.shows_processed}</td>
            <td>{log.episodes_updated}</td>
            <td>
              <span style="text-transform:capitalize;font-size:12px;color:var(--text-secondary)">
                {log.trigger || log.trigger_type || '-'}
              </span>
            </td>
            <td><Badge type={statusType(log.status)} label={log.status} /></td>
          </tr>

          {#if expandedId === log.id}
            <tr class="expanded-content">
              <td colspan="7">
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;max-width:600px">
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Started At</div>
                    <div style="font-size:13px">{formatTime(log.started_at)}</div>
                  </div>
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Completed At</div>
                    <div style="font-size:13px">{formatTime(log.completed_at)}</div>
                  </div>
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Shows Processed</div>
                    <div style="font-size:13px">{log.shows_processed}</div>
                  </div>
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Episodes Updated</div>
                    <div style="font-size:13px">{log.episodes_updated}</div>
                  </div>
                </div>
                {#if log.errors}
                  <div style="margin-top:12px">
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Errors</div>
                    <div style="font-size:12px;color:var(--danger);background:var(--danger-bg);padding:8px 12px;border-radius:var(--radius);font-family:monospace;white-space:pre-wrap">{log.errors}</div>
                  </div>
                {/if}
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}
'@ | Set-Content -Path "web\src\pages\Logs.svelte" -Encoding UTF8

# --- web/src/pages/Settings.svelte ---
@'
<script>
  import { onMount } from 'svelte';
  import { settings, addNotification } from '../lib/stores.js';
  import { getSettings, updateSettings, testSonarrConnection } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';

  let loading = true;
  let saving = false;
  let testing = false;
  let testResult = null;

  let sonarrUrl = '';
  let sonarrApiKey = '';
  let defaultFillerAction = 'unmonitor';
  let defaultMixedAction = 'monitor';
  let syncInterval = '6h';
  let enableTagging = false;
  let fillerTagName = 'filler';
  let cacheTtl = '24h';

  const syncIntervalOptions = [
    { value: '1h', label: 'Every 1 hour' },
    { value: '3h', label: 'Every 3 hours' },
    { value: '6h', label: 'Every 6 hours' },
    { value: '12h', label: 'Every 12 hours' },
    { value: '24h', label: 'Every 24 hours' },
  ];

  const cacheTtlOptions = [
    { value: '1h', label: '1 hour' },
    { value: '6h', label: '6 hours' },
    { value: '12h', label: '12 hours' },
    { value: '24h', label: '24 hours' },
    { value: '48h', label: '48 hours' },
    { value: '168h', label: '7 days' },
  ];

  onMount(async () => {
    await loadSettings();
  });

  async function loadSettings() {
    loading = true;
    try {
      const data = await getSettings();
      if (data && !data.error) {
        sonarrUrl = data.sonarr_url || '';
        sonarrApiKey = data.sonarr_api_key || '';
        defaultFillerAction = data.default_filler_action || 'unmonitor';
        defaultMixedAction = data.default_mixed_action || 'monitor';
        syncInterval = data.sync_interval || '6h';
        enableTagging = data.enable_tagging || false;
        fillerTagName = data.filler_tag_name || 'filler';
        cacheTtl = data.cache_ttl || '24h';
        settings.set(data);
      }
    } catch (err) {
      // Settings endpoint may not be implemented yet
    }
    loading = false;
  }

  async function handleSave() {
    saving = true;
    try {
      const data = {
        sonarr_url: sonarrUrl,
        sonarr_api_key: sonarrApiKey,
        default_filler_action: defaultFillerAction,
        default_mixed_action: defaultMixedAction,
        sync_interval: syncInterval,
        enable_tagging: enableTagging,
        filler_tag_name: fillerTagName,
        cache_ttl: cacheTtl,
      };
      await updateSettings(data);
      settings.set(data);
      addNotification('Settings saved successfully', 'success');
    } catch (err) {
      addNotification('Failed to save settings: ' + err.message, 'error');
    }
    saving = false;
  }

  async function handleTestConnection() {
    if (!sonarrUrl) {
      addNotification('Please enter a Sonarr URL first', 'warning');
      return;
    }
    testing = true;
    testResult = null;
    try {
      const result = await testSonarrConnection(sonarrUrl, sonarrApiKey);
      testResult = { success: true, message: 'Connection successful!' };
      addNotification('Sonarr connection test passed', 'success');
    } catch (err) {
      testResult = { success: false, message: err.message };
      addNotification('Sonarr connection test failed: ' + err.message, 'error');
    }
    testing = false;
  }
</script>

<div class="page-header">
  <div>
    <h2>Settings</h2>
    <p>Configure AnimeFillerGuard connection and behavior</p>
  </div>
  <button class="btn btn-primary" on:click={handleSave} disabled={saving}>
    {#if saving}
      <span class="spinner"></span> Saving...
    {:else}
      Save Settings
    {/if}
  </button>
</div>

{#if loading}
  <Spinner center size="lg" message="Loading settings..." />
{:else}
  <!-- Sonarr Connection -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Sonarr Connection</h3>
        {#if testResult}
          <span style="font-size:12px;color:{testResult.success ? 'var(--success)' : 'var(--danger)'}">
            {testResult.message}
          </span>
        {/if}
      </div>

      <div class="form-group">
        <label class="form-label" for="sonarr-url">Sonarr URL</label>
        <input
          id="sonarr-url"
          class="form-input"
          type="text"
          placeholder="http://localhost:8989"
          bind:value={sonarrUrl}
        />
        <div class="form-hint">Full URL to your Sonarr instance (e.g., http://192.168.1.100:8989)</div>
      </div>

      <div class="form-group">
        <label class="form-label" for="sonarr-key">API Key</label>
        <input
          id="sonarr-key"
          class="form-input"
          type="password"
          placeholder="Your Sonarr API key"
          bind:value={sonarrApiKey}
        />
        <div class="form-hint">Found in Sonarr under Settings &rarr; General &rarr; Security</div>
      </div>

      <button class="btn btn-secondary" on:click={handleTestConnection} disabled={testing}>
        {#if testing}
          <span class="spinner"></span> Testing...
        {:else}
          Test Connection
        {/if}
      </button>
    </div>
  </div>

  <!-- Default Filler Rules -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Default Filler Rules</h3>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label class="form-label" for="default-filler">Default Filler Action</label>
          <select id="default-filler" class="form-select" bind:value={defaultFillerAction}>
            <option value="unmonitor">Unmonitor</option>
            <option value="monitor">Monitor (keep)</option>
            <option value="skip">Skip & Delete</option>
          </select>
          <div class="form-hint">Applied to filler episodes for newly mapped shows</div>
        </div>

        <div class="form-group">
          <label class="form-label" for="default-mixed">Default Mixed Canon Action</label>
          <select id="default-mixed" class="form-select" bind:value={defaultMixedAction}>
            <option value="monitor">Monitor (keep)</option>
            <option value="unmonitor">Unmonitor</option>
            <option value="skip">Skip & Delete</option>
          </select>
          <div class="form-hint">Applied to mixed canon/filler episodes</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Sync Settings -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Sync Settings</h3>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label class="form-label" for="sync-interval">Sync Interval</label>
          <select id="sync-interval" class="form-select" bind:value={syncInterval}>
            {#each syncIntervalOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
          <div class="form-hint">How often to sync filler data from AnimeFillerList</div>
        </div>

        <div class="form-group">
          <label class="form-label" for="cache-ttl">Cache TTL</label>
          <select id="cache-ttl" class="form-select" bind:value={cacheTtl}>
            {#each cacheTtlOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
          <div class="form-hint">How long to cache scraped AFL data before refreshing</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Tag Management -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Tag Management</h3>
      </div>

      <div class="form-group">
        <div style="display:flex;align-items:center;gap:12px">
          <label class="toggle">
            <input type="checkbox" bind:checked={enableTagging} />
            <span class="toggle-slider"></span>
          </label>
          <div>
            <div style="font-size:13px;font-weight:500">Enable Filler Tagging</div>
            <div class="form-hint" style="margin-top:2px">Add a tag to series in Sonarr that have filler episodes</div>
          </div>
        </div>
      </div>

      {#if enableTagging}
        <div class="form-group">
          <label class="form-label" for="tag-name">Tag Name</label>
          <input
            id="tag-name"
            class="form-input"
            type="text"
            style="max-width:240px"
            placeholder="filler"
            bind:value={fillerTagName}
          />
          <div class="form-hint">The tag label to apply in Sonarr</div>
        </div>
      {/if}
    </div>
  </div>
{/if}
'@ | Set-Content -Path "web\src\pages\Settings.svelte" -Encoding UTF8

# --- web/src/pages/ShowDetail.svelte ---
@'
<script>
  import { onMount } from 'svelte';
  import { push } from 'svelte-spa-router';
  import { addNotification } from '../lib/stores.js';
  import { getShows, updateShow, getEpisodes } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';

  export let params = {};

  let loading = true;
  let saving = false;
  let show = null;
  let episodes = [];
  let fillerAction = 'unmonitor';
  let mixedAction = 'monitor';
  let episodeOffset = 0;
  let enabled = true;
  let epFilter = 'all'; // all, canon, mixed_canon, filler, unknown

  $: showId = params.id;

  $: filteredEpisodes = episodes.filter(ep => {
    if (epFilter === 'all') return true;
    return ep.type === epFilter;
  });

  $: epStats = {
    total: episodes.length,
    canon: episodes.filter(e => e.type === 'canon').length,
    mixed: episodes.filter(e => e.type === 'mixed_canon').length,
    filler: episodes.filter(e => e.type === 'filler').length,
    unknown: episodes.filter(e => e.type === 'unknown').length,
  };

  onMount(async () => {
    await loadShow();
  });

  async function loadShow() {
    loading = true;
    try {
      const allShows = await getShows();
      show = (allShows || []).find(s => String(s.id) === String(showId));
      if (!show) {
        addNotification('Show not found', 'error');
        push('/shows');
        return;
      }
      fillerAction = show.filler_action;
      mixedAction = show.mixed_action;
      episodeOffset = show.episode_offset;
      enabled = show.enabled;

      try {
        episodes = await getEpisodes(showId) || [];
      } catch {
        episodes = [];
      }
    } catch (err) {
      addNotification('Failed to load show: ' + err.message, 'error');
    }
    loading = false;
  }

  async function handleSave() {
    saving = true;
    try {
      await updateShow(showId, {
        filler_action: fillerAction,
        mixed_action: mixedAction,
        episode_offset: parseInt(episodeOffset) || 0,
        enabled: enabled,
      });
      addNotification('Show settings saved', 'success');
    } catch (err) {
      addNotification('Failed to save: ' + err.message, 'error');
    }
    saving = false;
  }

  function applyBulkAction(action) {
    switch (action) {
      case 'skip_filler':
        fillerAction = 'unmonitor';
        mixedAction = 'monitor';
        break;
      case 'skip_filler_and_mixed':
        fillerAction = 'unmonitor';
        mixedAction = 'unmonitor';
        break;
      case 'keep_all':
        fillerAction = 'monitor';
        mixedAction = 'monitor';
        break;
    }
    addNotification(`Applied rule: ${action.replace(/_/g, ' ')}`, 'info');
  }

  function epRowClass(type) {
    switch (type) {
      case 'canon': return 'ep-canon';
      case 'mixed_canon': return 'ep-mixed';
      case 'anime_canon': return 'ep-anime-canon';
      case 'filler': return 'ep-filler';
      default: return '';
    }
  }

  function confidenceClass(conf) {
    if (conf >= 0.8) return 'high';
    if (conf >= 0.5) return 'medium';
    return 'low';
  }

  function formatAction(action) {
    const map = {
      unmonitor: 'Unmonitor',
      monitor: 'Monitor',
      skip: 'Skip & Delete',
    };
    return map[action] || action;
  }
</script>

{#if loading}
  <Spinner center size="lg" message="Loading show details..." />
{:else if !show}
  <div class="empty-state">
    <h3>Show Not Found</h3>
    <p>The requested show mapping could not be found.</p>
    <a href="#/shows" class="btn btn-primary">Back to Shows</a>
  </div>
{:else}
  <div class="page-header">
    <div>
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px">
        <a href="#/shows" style="color:var(--text-muted);font-size:13px">&larr; Shows</a>
      </div>
      <h2>{show.sonarr_title}</h2>
      <p>Mapped to {show.afl_title || show.afl_slug}</p>
    </div>
    <div class="btn-group">
      <button class="btn btn-primary" on:click={handleSave} disabled={saving}>
        {#if saving}
          <span class="spinner"></span> Saving...
        {:else}
          Save Changes
        {/if}
      </button>
    </div>
  </div>

  <!-- Show Info Headers -->
  <div class="show-header">
    <div class="show-info-card">
      <h3>Sonarr Info</h3>
      <div class="info-row">
        <span class="info-label">Series ID</span>
        <span class="info-value">{show.sonarr_series_id}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Title</span>
        <span class="info-value">{show.sonarr_title}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Status</span>
        <span class="info-value">
          {#if enabled}
            <Badge type="success" label="Enabled" />
          {:else}
            <Badge type="unknown" label="Disabled" />
          {/if}
        </span>
      </div>
      <div class="info-row">
        <span class="info-label">Created</span>
        <span class="info-value">{new Date(show.created_at).toLocaleDateString()}</span>
      </div>
    </div>

    <div class="show-info-card">
      <h3>AnimeFillerList Info</h3>
      <div class="info-row">
        <span class="info-label">Title</span>
        <span class="info-value">{show.afl_title}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Slug</span>
        <span class="info-value">{show.afl_slug}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Match Confidence</span>
        <span class="info-value">
          <span class="confidence-bar">
            <span
              class="confidence-fill {confidenceClass(show.match_confidence)}"
              style="width:{Math.round(show.match_confidence * 100)}%"
            ></span>
          </span>
          {Math.round(show.match_confidence * 100)}%
        </span>
      </div>
      <div class="info-row">
        <span class="info-label">Auto-Matched</span>
        <span class="info-value">{show.auto_matched ? 'Yes' : 'No'}</span>
      </div>
    </div>
  </div>

  <!-- Filler Rules & Controls -->
  <div class="controls-bar">
    <div style="display:flex;align-items:center;gap:16px;flex-wrap:wrap">
      <div class="form-group" style="margin:0">
        <label class="form-label" style="margin-bottom:4px" for="filler-rule">Filler Rule</label>
        <select id="filler-rule" class="form-select" style="width:160px" bind:value={fillerAction}>
          <option value="unmonitor">Unmonitor</option>
          <option value="monitor">Monitor</option>
          <option value="skip">Skip & Delete</option>
        </select>
      </div>

      <div class="form-group" style="margin:0">
        <label class="form-label" style="margin-bottom:4px" for="mixed-rule">Mixed Rule</label>
        <select id="mixed-rule" class="form-select" style="width:160px" bind:value={mixedAction}>
          <option value="unmonitor">Unmonitor</option>
          <option value="monitor">Monitor</option>
          <option value="skip">Skip & Delete</option>
        </select>
      </div>

      <div class="form-group" style="margin:0">
        <label class="form-label" style="margin-bottom:4px" for="ep-offset">Episode Offset</label>
        <input id="ep-offset" class="form-input" type="number" style="width:90px" bind:value={episodeOffset} />
      </div>

      <div class="form-group" style="margin:0">
        <label class="form-label" style="margin-bottom:4px">Enabled</label>
        <label class="toggle">
          <input type="checkbox" bind:checked={enabled} />
          <span class="toggle-slider"></span>
        </label>
      </div>
    </div>

    <div class="btn-group">
      <button class="btn btn-sm btn-secondary" on:click={() => applyBulkAction('skip_filler')}>
        Skip Filler Only
      </button>
      <button class="btn btn-sm btn-secondary" on:click={() => applyBulkAction('skip_filler_and_mixed')}>
        Skip Filler + Mixed
      </button>
      <button class="btn btn-sm btn-secondary" on:click={() => applyBulkAction('keep_all')}>
        Monitor All
      </button>
    </div>
  </div>

  <!-- Episode Stats -->
  <div class="stats-grid" style="margin-bottom:16px">
    <div class="stat-card" style="padding:12px 16px">
      <span class="stat-label">Total</span>
      <span class="stat-value" style="font-size:20px">{epStats.total}</span>
    </div>
    <div class="stat-card" style="padding:12px 16px;cursor:pointer" on:click={() => (epFilter = epFilter === 'canon' ? 'all' : 'canon')} class:active-filter={epFilter === 'canon'}>
      <span class="stat-label" style="color:var(--canon)">Canon</span>
      <span class="stat-value" style="font-size:20px;color:var(--canon)">{epStats.canon}</span>
    </div>
    <div class="stat-card" style="padding:12px 16px;cursor:pointer" on:click={() => (epFilter = epFilter === 'mixed_canon' ? 'all' : 'mixed_canon')} class:active-filter={epFilter === 'mixed_canon'}>
      <span class="stat-label" style="color:var(--mixed)">Mixed</span>
      <span class="stat-value" style="font-size:20px;color:var(--mixed)">{epStats.mixed}</span>
    </div>
    <div class="stat-card" style="padding:12px 16px;cursor:pointer" on:click={() => (epFilter = epFilter === 'filler' ? 'all' : 'filler')} class:active-filter={epFilter === 'filler'}>
      <span class="stat-label" style="color:var(--filler)">Filler</span>
      <span class="stat-value" style="font-size:20px;color:var(--filler)">{epStats.filler}</span>
    </div>
  </div>

  <!-- Episode List -->
  {#if episodes.length === 0}
    <div class="card">
      <div class="empty-state">
        <div class="empty-icon">&#128196;</div>
        <h3>No Episode Data</h3>
        <p>Episode data will appear after the first sync with AnimeFillerList.</p>
      </div>
    </div>
  {:else}
    <div class="table-container">
      <table>
        <thead>
          <tr>
            <th style="width:70px">#</th>
            <th>Title</th>
            <th style="width:120px">Type</th>
            <th style="width:120px">Arc</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredEpisodes as ep}
            <tr class={epRowClass(ep.type)}>
              <td style="font-weight:600;font-variant-numeric:tabular-nums">{ep.episode_num}</td>
              <td>{ep.title || '-'}</td>
              <td><Badge type={ep.type} /></td>
              <td style="font-size:12px;color:var(--text-muted)">{ep.arc_name || '-'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if epFilter !== 'all'}
      <p style="margin-top:8px;font-size:12px;color:var(--text-muted)">
        Showing {filteredEpisodes.length} of {episodes.length} episodes (filtered by {epFilter.replace('_', ' ')})
      </p>
    {/if}
  {/if}
{/if}

<style>
  .active-filter {
    border-color: var(--accent) !important;
    background: var(--accent-muted) !important;
  }
</style>
'@ | Set-Content -Path "web\src\pages\ShowDetail.svelte" -Encoding UTF8

# --- web/src/pages/Shows.svelte ---
@'
<script>
  import { onMount } from 'svelte';
  import { push } from 'svelte-spa-router';
  import { shows, addNotification } from '../lib/stores.js';
  import { getShows, deleteShow, searchAFL, autoMatch, confirmMatch } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import Modal from '../components/Modal.svelte';

  let loading = true;
  let filter = '';
  let showAddModal = false;
  let addStep = 'search'; // 'search' | 'results' | 'confirm'
  let aflQuery = '';
  let aflResults = [];
  let aflSearching = false;
  let selectedAfl = null;
  let addingSonarrId = null;
  let confirming = false;

  $: filteredShows = ($shows || []).filter(s => {
    if (!filter) return true;
    const q = filter.toLowerCase();
    return (s.sonarr_title || '').toLowerCase().includes(q)
      || (s.afl_title || '').toLowerCase().includes(q);
  });

  onMount(async () => {
    await loadShows();
  });

  async function loadShows() {
    loading = true;
    try {
      const data = await getShows();
      shows.set(data || []);
    } catch (err) {
      addNotification('Failed to load shows: ' + err.message, 'error');
    }
    loading = false;
  }

  async function handleDelete(show) {
    if (!confirm(`Unmap "${show.sonarr_title}"? This will remove the AFL mapping.`)) return;
    try {
      await deleteShow(show.id);
      addNotification(`Unmapped "${show.sonarr_title}"`, 'success');
      await loadShows();
    } catch (err) {
      addNotification('Failed to unmap: ' + err.message, 'error');
    }
  }

  function openAddModal() {
    showAddModal = true;
    addStep = 'search';
    aflQuery = '';
    aflResults = [];
    selectedAfl = null;
  }

  async function handleAflSearch() {
    if (!aflQuery.trim()) return;
    aflSearching = true;
    try {
      aflResults = await searchAFL(aflQuery);
      addStep = 'results';
    } catch (err) {
      addNotification('AFL search failed: ' + err.message, 'error');
    }
    aflSearching = false;
  }

  async function handleConfirmMatch() {
    if (!selectedAfl) return;
    confirming = true;
    try {
      await confirmMatch(addingSonarrId, selectedAfl.slug);
      addNotification('Show mapped successfully!', 'success');
      showAddModal = false;
      await loadShows();
    } catch (err) {
      addNotification('Failed to map show: ' + err.message, 'error');
    }
    confirming = false;
  }

  function confidenceClass(conf) {
    if (conf >= 0.8) return 'high';
    if (conf >= 0.5) return 'medium';
    return 'low';
  }

  function formatAction(action) {
    const map = {
      unmonitor: 'Unmonitor',
      monitor: 'Monitor',
      skip: 'Skip & Delete',
    };
    return map[action] || action;
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleDateString();
  }
</script>

<div class="page-header">
  <div>
    <h2>Shows</h2>
    <p>Manage Sonarr to AnimeFillerList mappings</p>
  </div>
  <button class="btn btn-primary" on:click={openAddModal}>+ Add Show</button>
</div>

<!-- Search Filter -->
<div style="margin-bottom:16px">
  <div class="search-bar">
    <span class="search-icon">&#128269;</span>
    <input
      class="form-input"
      type="text"
      placeholder="Filter shows..."
      bind:value={filter}
    />
  </div>
</div>

{#if loading}
  <Spinner center size="lg" message="Loading shows..." />
{:else if filteredShows.length === 0}
  <div class="card">
    <div class="empty-state">
      <div class="empty-icon">&#128253;</div>
      <h3>{filter ? 'No Matching Shows' : 'No Shows Mapped'}</h3>
      <p>
        {filter
          ? 'Try a different search term.'
          : 'Click "Add Show" to map a Sonarr anime series to AnimeFillerList.'}
      </p>
      {#if !filter}
        <button class="btn btn-primary" on:click={openAddModal}>+ Add Show</button>
      {/if}
    </div>
  </div>
{:else}
  <div class="table-container">
    <table>
      <thead>
        <tr>
          <th>Sonarr Title</th>
          <th>AFL Match</th>
          <th>Confidence</th>
          <th>Filler Rule</th>
          <th>Mixed Rule</th>
          <th>Enabled</th>
          <th>Updated</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredShows as show}
          <tr>
            <td>
              <a href="#/shows/{show.id}" style="color:var(--text-primary);font-weight:500">
                {show.sonarr_title}
              </a>
            </td>
            <td style="color:var(--text-secondary)">{show.afl_title || show.afl_slug}</td>
            <td>
              <div style="display:flex;align-items:center;gap:4px">
                <span class="confidence-bar">
                  <span
                    class="confidence-fill {confidenceClass(show.match_confidence)}"
                    style="width:{Math.round(show.match_confidence * 100)}%"
                  ></span>
                </span>
                <span style="font-size:12px;color:var(--text-muted)">
                  {Math.round(show.match_confidence * 100)}%
                </span>
              </div>
            </td>
            <td><span style="font-size:12px">{formatAction(show.filler_action)}</span></td>
            <td><span style="font-size:12px">{formatAction(show.mixed_action)}</span></td>
            <td>
              {#if show.enabled}
                <Badge type="success" label="Active" />
              {:else}
                <Badge type="unknown" label="Disabled" />
              {/if}
            </td>
            <td style="font-size:12px;color:var(--text-muted)">{formatTime(show.updated_at)}</td>
            <td>
              <div class="btn-group">
                <button class="btn btn-sm btn-secondary" on:click={() => push(`/shows/${show.id}`)}>
                  Edit
                </button>
                <button class="btn btn-sm btn-danger" on:click={() => handleDelete(show)}>
                  Unmap
                </button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<!-- Add Show Modal -->
<Modal title="Add Show Mapping" show={showAddModal} on:close={() => (showAddModal = false)} size="large">
  {#if addStep === 'search'}
    <p style="font-size:13px;color:var(--text-secondary);margin-bottom:16px">
      Search AnimeFillerList by anime title to find a match for your Sonarr series.
    </p>
    <div class="form-group">
      <label class="form-label" for="afl-search">Search AnimeFillerList</label>
      <div style="display:flex;gap:8px">
        <input
          id="afl-search"
          class="form-input"
          type="text"
          placeholder="e.g. Naruto Shippuden"
          bind:value={aflQuery}
          on:keydown={(e) => e.key === 'Enter' && handleAflSearch()}
        />
        <button class="btn btn-primary" on:click={handleAflSearch} disabled={aflSearching}>
          {#if aflSearching}
            <span class="spinner"></span>
          {:else}
            Search
          {/if}
        </button>
      </div>
    </div>

  {:else if addStep === 'results'}
    <button class="btn btn-sm btn-secondary" style="margin-bottom:12px" on:click={() => (addStep = 'search')}>
      &larr; Back to Search
    </button>

    {#if aflResults.length === 0}
      <div class="empty-state" style="padding:20px">
        <h3>No Results</h3>
        <p>No shows found on AnimeFillerList matching "{aflQuery}". Try a different term.</p>
      </div>
    {:else}
      <p style="font-size:13px;color:var(--text-secondary);margin-bottom:12px">
        Found {aflResults.length} result{aflResults.length === 1 ? '' : 's'}. Select a match:
      </p>
      <div style="display:flex;flex-direction:column;gap:6px">
        {#each aflResults as result}
          <button
            class="card"
            style="padding:12px 16px;cursor:pointer;text-align:left;border:1px solid {selectedAfl?.slug === result.slug ? 'var(--accent)' : 'var(--border)'};background:{selectedAfl?.slug === result.slug ? 'var(--accent-muted)' : 'var(--bg-secondary)'}"
            on:click={() => (selectedAfl = result)}
          >
            <div style="font-weight:600;font-size:14px">{result.title}</div>
            <div style="font-size:12px;color:var(--text-muted);margin-top:2px">{result.slug}</div>
          </button>
        {/each}
      </div>
    {/if}

  {:else if addStep === 'confirm'}
    <p>Confirm mapping to <strong>{selectedAfl.title}</strong>?</p>
  {/if}

  <svelte:fragment slot="footer">
    <button class="btn btn-secondary" on:click={() => (showAddModal = false)}>Cancel</button>
    {#if addStep === 'results' && selectedAfl}
      <button class="btn btn-primary" on:click={handleConfirmMatch} disabled={confirming}>
        {#if confirming}
          <span class="spinner"></span> Mapping...
        {:else}
          Confirm Match
        {/if}
      </button>
    {/if}
  </svelte:fragment>
</Modal>
'@ | Set-Content -Path "web\src\pages\Shows.svelte" -Encoding UTF8

# --- web/svelte.config.js ---
@'
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
};

'@ | Set-Content -Path "web\svelte.config.js" -Encoding UTF8

# --- web/vite.config.js ---
@'
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8978',
        changeOrigin: true,
      },
    },
  },
});
'@ | Set-Content -Path "web\vite.config.js" -Encoding UTF8

Write-Host ""
Write-Host "==> AnimeFillerGuard project created successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "    To get started:" -ForegroundColor Yellow
Write-Host "      1. cd animefillerguard"
Write-Host "      2. Build frontend:  cd web; npm install; npm run build; cd .."
Write-Host "      3. Run go mod tidy"
Write-Host "      4. go build -o animefillerguard.exe .\cmd\server"
Write-Host "      5. Or: docker compose up -d"
Write-Host ""
