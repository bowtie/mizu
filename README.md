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
cd web && npm install && npm run build && cd ..

# Build Go binary
go build -o mizu ./cmd/server
```

## Configuration

All settings are stored in the embedded SQLite database and managed through the web UI (Settings page). Defaults are applied on first launch and can be overridden at any time.

### Settings

| Setting | Default | Description |
|---|---|---|
| `sonarr_url` | `http://localhost:8989` | Sonarr instance URL |
| `sonarr_api_key` | *(required)* | Sonarr API key (Settings > General > Security) |
| `sync_interval` | `6h` | How often to sync (1h, 3h, 6h, 12h, or 24h) |
| `default_filler_action` | `unmonitor` | Default action for filler episodes |
| `default_mixed_canon_action` | `monitor` | Default action for mixed canon episodes |
| `default_anime_canon_action` | `monitor` | Default action for anime canon episodes |
| `enable_anilist_artwork` | `false` | Fetch poster/banner artwork from AniList |
| `ntfy_enabled` | `false` | Enable ntfy push notifications |
| `ntfy_server_url` | `https://ntfy.sh` | ntfy server URL |
| `ntfy_topic` | *(empty)* | ntfy topic name |
| `ntfy_token` | *(empty)* | ntfy access token (optional) |

### Filler Actions

| Action | Behavior |
|---|---|
| `unmonitor` | Marks filler episodes as unmonitored in Sonarr (won't be downloaded) |
| `monitor` | Keeps episodes monitored (no change) |

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
  web/embed.go         # Embeds Svelte SPA into Go binary

web/                   # Svelte SPA frontend
  src/
    components/        # Navbar, Modal, Toast, Badge, Spinner
    pages/             # Dashboard, Shows, ShowDetail, Settings, Logs
    lib/               # API client, stores
```

## Notifications (ntfy)

Mizu can send push notifications after each sync via [ntfy.sh](https://ntfy.sh).

### Setup

1. Install the ntfy app on your phone ([Android](https://play.google.com/store/apps/details?id=io.heckel.ntfy), [iOS](https://apps.apple.com/app/ntfy/id1625396347))
2. Subscribe to your chosen topic (e.g., `mizu`)
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
cd web && vp dev

# Backend
go run ./cmd/server

# Tests
cd web && vp test

# Lint + format + type-check
cd web && vp check

# Production build
cd web && vp build
```

`bun run dev` opens two PowerShell windows:
- frontend Vite+ dev server on http://localhost:5173
- backend watcher that restarts `go run ./cmd/server` whenever Go files change

The Vite+ dev server proxies /api requests to the Go backend on port 8978.

## License

MIT

