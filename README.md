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
