# Mizu

Automatically manage filler episodes in Sonarr using data from [AnimeFillerList](https://www.animefillerlist.com/). Unmonitor or keep filler and mixed-canon episodes so you only download the episodes that matter.

## Features

- **Automatic filler detection** -- Scrapes AnimeFillerList and cross-references with your Sonarr library
- **Fuzzy matching** -- Jaro-Winkler algorithm matches Sonarr series to AFL entries, even with title variations
- **Configurable actions** -- Per-show rules: unmonitor or keep (monitor) filler episodes
- **Mixed canon support** -- Separate rules for mixed canon/filler episodes
- **Push notifications** -- Get notified on sync results via [ntfy.sh](https://ntfy.sh) (self-hosted or public)
- **Web dashboard** -- Dark-themed UI for managing mappings, viewing episodes, and configuring settings
- **Scheduled sync** -- Automatic periodic sync with configurable intervals
- **Docker-ready** -- Single container with embedded SQLite database

## Quick Start

### Docker Compose (Recommended)

```yaml
services:
  mizu:
    image: mizu:latest
    container_name: mizu
    ports:
      - "8978:8978"
    volumes:
      - ./config:/config
    restart: unless-stopped
```

```bash
docker compose up -d
```

Open `http://localhost:8978` to access the dashboard, then configure your Sonarr URL and API key in Settings.

### Docker Run

```bash
docker run -d \
  --name mizu \
  -p 8978:8978 \
  -v ./config:/config \
  mizu:latest
```

### Build from Source

```bash
# Build frontend
cd web-astryx && bun install && bun run build && cd ..

# Build Go binary
go build -o mizu ./cmd/server
```

## Configuration

All settings are stored in the embedded SQLite database and managed through the web UI (Settings page). Defaults are applied on first launch and can be overridden at any time.

### Settings

| Setting                      | Default                 | Description                                                               |
| ---------------------------- | ----------------------- | ------------------------------------------------------------------------- |
| `sonarr_url`                 | `http://localhost:8989` | Sonarr instance URL                                                       |
| `sonarr_api_key`             | _(required)_            | Sonarr API key (Settings > General > Security)                            |
| `sync_interval`              | `6h`                    | How often to sync (1h, 3h, 6h, 12h, or 24h)                               |
| `default_filler_action`      | `unmonitor`             | Default action for filler episodes                                        |
| `default_mixed_canon_action` | `monitor`               | Default action for mixed canon episodes                                   |
| `default_anime_canon_action` | `monitor`               | Default action for anime canon episodes                                   |
| `artwork_default_source`     | `sonarr`                | Default artwork provider (`sonarr`, `anilist`, `tmdb`, `fanart`, `kitsu`) |
| `ntfy_enabled`               | `false`                 | Enable ntfy push notifications                                            |
| `ntfy_server_url`            | `https://ntfy.sh`       | ntfy server URL                                                           |
| `ntfy_topic`                 | _(empty)_               | ntfy topic name                                                           |
| `ntfy_token`                 | _(empty)_               | ntfy access token (optional)                                              |

### Filler Actions

| Action      | Behavior                                                             |
| ----------- | -------------------------------------------------------------------- |
| `unmonitor` | Marks filler episodes as unmonitored in Sonarr (won't be downloaded) |
| `monitor`   | Keeps episodes monitored (no change)                                 |

## Architecture

```
cmd/server/
  main.go              # Entry point, wires up all components
  router.go            # HTTP API routes and handlers

internal/
  config/config.go     # SQLite-based configuration with defaults
  database/database.go # SQLite with migrations, full CRUD
  matcher/matcher.go   # Jaro-Winkler fuzzy title matching
  models/models.go     # Shared types (Sonarr, AFL, mappings)
  notifier/ntfy.go     # ntfy.sh push notifications
  scraper/scraper.go   # AnimeFillerList HTML parser
  sonarr/sonarr.go     # Sonarr API v3 client
  sync/sync.go         # Orchestrates scraper + matcher + Sonarr
  web/embed.go         # Embeds React SPA into Go binary

web-astryx/            # React SPA frontend (Vite+ + StyleX)
  src/
    routes/            # ShowsPage, ShowDetailPage, SettingsPage, ActivityPage, AddAnimeDialog
    components/        # ToastViewport
    lib/               # API client, stores, types, themes
```

## Notifications (ntfy)

Mizu can send push notifications after each sync via [ntfy.sh](https://ntfy.sh).

### Setup

1. Install the ntfy app on your phone ([Android](https://play.google.com/store/apps/details?id=io.heckel.ntfy), [iOS](https://apps.apple.com/app/ntfy/id1625396347))
2. Subscribe to your chosen topic (e.g., `mizu`)
3. In Settings > Notifications, enable ntfy and enter the same topic
4. Click "Send Test Notification" to verify

### Self-hosted ntfy

If you run your own ntfy server, set the ntfy server URL in Settings and provide an access token if authentication is required.

### Notification Types

| Status  | Priority    | When                                   |
| ------- | ----------- | -------------------------------------- |
| Success | Default (3) | All shows synced without errors        |
| Partial | High (4)    | Some shows failed but others succeeded |
| Failed  | Urgent (5)  | Sync completely failed                 |

## API Reference

All endpoints are under `/api`.

| Method   | Path                                        | Description                                             |
| -------- | ------------------------------------------- | ------------------------------------------------------- |
| `GET`    | `/api/health`                               | Health check with Sonarr connectivity                   |
| `GET`    | `/api/version`                              | Version info                                            |
| `GET`    | `/api/stats`                                | Dashboard statistics                                    |
| `GET`    | `/api/artwork/providers`                    | List available artwork providers                        |
| `GET`    | `/api/artwork/{id}/poster`                  | Serve poster artwork for a mapping                      |
| `GET`    | `/api/artwork/{id}/banner`                  | Serve banner artwork for a mapping                      |
| `GET`    | `/api/artwork/{id}/logo`                    | Serve logo artwork for a mapping                        |
| `POST`   | `/api/artwork/refresh-all`                  | Refresh artwork for all mappings                        |
| `GET`    | `/api/mappings`                             | List all show mappings                                  |
| `POST`   | `/api/mappings`                             | Create mapping (auto-match or manual)                   |
| `GET`    | `/api/mappings/:id`                         | Get a single mapping                                    |
| `PUT`    | `/api/mappings/:id`                         | Update a mapping                                        |
| `DELETE` | `/api/mappings/:id`                         | Delete a mapping                                        |
| `POST`   | `/api/mappings/:id/artwork/refresh`         | Refresh artwork for a single mapping                    |
| `PUT`    | `/api/mappings/:id/artwork/source`          | Change artwork source for a mapping                     |
| `GET`    | `/api/mappings/:id/episodes`                | Get episodes for a mapping                              |
| `GET`    | `/api/mappings/:id/episodes/sonarr`         | Get Sonarr episode data for a mapping                   |
| `PUT`    | `/api/mappings/:id/episodes/{epId}/monitor` | Toggle episode monitor status                           |
| `POST`   | `/api/mappings/:id/resync`                  | Resync a single mapping                                 |
| `POST`   | `/api/sync`                                 | Trigger manual sync                                     |
| `GET`    | `/api/logs`                                 | Get sync history                                        |
| `GET`    | `/api/settings`                             | Get current settings                                    |
| `GET`    | `/api/settings/sonarr-api-key`              | Get stored Sonarr API key                               |
| `PUT`    | `/api/settings`                             | Update settings                                         |
| `POST`   | `/api/settings/afl-cache/refresh`           | Refresh AFL cache                                       |
| `GET`    | `/api/sonarr/series`                        | List Sonarr series                                      |
| `GET`    | `/api/sonarr/series/:id`                    | Get a single Sonarr series                              |
| `POST`   | `/api/sonarr/series`                        | Add a series to Sonarr                                  |
| `GET`    | `/api/sonarr/lookup`                        | Lookup series in Sonarr                                 |
| `GET`    | `/api/sonarr/add-options`                   | Get Sonarr add options (root folders, quality profiles) |
| `POST`   | `/api/sonarr/manual-match-pending`          | Mark a series as pending manual match                   |
| `POST`   | `/api/sonarr/test`                          | Test Sonarr connection                                  |
| `POST`   | `/api/ntfy/test`                            | Send test ntfy notification                             |
| `GET`    | `/api/afl/search?q=`                        | Search AnimeFillerList                                  |
| `GET`    | `/api/afl/suggest`                          | Get AFL suggestions                                     |

## Development

This project uses [Vite+](https://viteplus.dev/) as its frontend toolchain. Install it first:

```powershell
irm https://vite.plus/ps1 | iex
```

```bash
# One command from the repo root (single terminal)
bun run dev

# Optional: keep the old two-window workflow
bun run dev:windows

# Frontend (hot reload via Vite+)
cd web-astryx && vp dev

# Backend
go run ./cmd/server

# Tests
go test ./...

# Tests with the race detector (requires cgo + a C compiler; on Windows add
# MinGW gcc to PATH, e.g. /c/msys64/mingw64/bin)
CGO_ENABLED=1 go test -race ./...

# Type-check
cd web-astryx && bunx tsc --noEmit

# Production build
cd web-astryx && bun run build
```

`bun run dev` starts both processes in one terminal:

- frontend Vite+ dev server on http://localhost:3001
- backend watcher (air) that restarts `go run ./cmd/server` whenever Go files change

The Vite+ dev server proxies /api requests to the Go backend on port 8978.

## License

MIT
