# Mizu

Automatically manage filler episodes in Sonarr using data from [AnimeFillerList](https://www.animefillerlist.com/). Unmonitor or keep filler and mixed-canon episodes so you only download the episodes that matter.

## Features

- **Automatic filler detection** -- Scrapes AnimeFillerList and cross-references with your Sonarr library
- **Fuzzy matching** -- Jaro-Winkler algorithm matches Sonarr series to AFL entries, even with title variations
- **Configurable actions** -- Per-show rules: unmonitor or keep (monitor) filler episodes
- **Mixed canon support** -- Separate rules for mixed canon/filler episodes
- **Dashboard password** -- Optional single-password protection with session cookies (*arr-style first-run setup)
- **Push notifications** -- Get notified on sync results via [ntfy.sh](https://ntfy.sh) (self-hosted or public)
- **Web dashboard** -- Dark-themed UI for managing mappings, viewing episodes, and configuring settings
- **Scheduled sync** -- Automatic periodic sync with configurable intervals
- **Docker-ready** -- Single container with embedded SQLite database

## Quick Start

There's no published image yet; both options below build it from this
repository.

### Docker Compose (Recommended)

```yaml
services:
  mizu:
    build: .
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

Open `http://localhost:8978`; the welcome screen asks for your Sonarr URL and API key.

### Docker Run

```bash
docker build -t mizu .
docker run -d \
  --name mizu \
  -p 8978:8978 \
  -v ./config:/config \
  mizu:latest
```

The container listens on `0.0.0.0:8978` by default. Bare binaries default to
`127.0.0.1:8978` (safe behind a same-host proxy) and can be overridden with the
`MIZU_ADDR` environment variable, e.g. `MIZU_ADDR=0.0.0.0:8978 ./mizu`.

### Build from Source

```bash
# Build frontend
cd web && bun install && bun run build && cd ..

# Copy the frontend into the Go embed directory
cp -r web/dist/. internal/web/dist/

# Build Go binary
go build -o mizu ./cmd/server
```

On Windows, `bun run build` from the repo root runs all three steps
(`scripts/build.ps1`). Without the copy step the binary still builds, but it
serves no dashboard.

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
| `ntfy_notify_on`             | `always`                | When to notify: `always`, `changes` (episodes updated or failures), or `failures` only |

### Filler Actions

| Action      | Behavior                                                             |
| ----------- | -------------------------------------------------------------------- |
| `unmonitor` | Marks filler episodes as unmonitored in Sonarr (won't be downloaded) |
| `monitor`   | Keeps episodes monitored (no change)                                 |

### Dashboard Password

Until a password is set, the dashboard (and API) is open to anyone who can reach it. On first launch the UI prompts you to create one; it is stored bcrypt-hashed in SQLite and can be changed or removed later in Settings (Security). The initial password can also be seeded non-interactively:

```bash
MIZU_PASSWORD=your-password ./mizu
```

The env var only applies while no password is configured; rotate via Settings afterwards. Sessions last 30 days, and changing the password revokes all existing sessions. Health and version endpoints stay public for monitoring.

### Behind a reverse proxy or auth proxy

If an auth proxy (tinyauth, Authelia, Authentik, oauth2-proxy, …) or your
reverse proxy already handles sign-in, you can leave Mizu's own password off.
Then the proxy is the only thing protecting Mizu, so make sure nothing can
reach Mizu except through it:

- Don't publish Mizu's port (`8978`) on the host. Put Mizu on the same
  Docker network as the proxy and let the proxy reach it there; with a bare
  binary, keep the default `127.0.0.1` listen address.
- Pass the public host through. Mizu rejects cross-site writes by checking
  the browser's `Origin` against the request's host, so the proxy must keep
  the original `Host` header or send it as `X-Forwarded-Host`. Caddy and
  Traefik do this by default; for nginx add:

  ```nginx
  proxy_set_header Host $host;
  proxy_set_header X-Forwarded-Proto $scheme;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  ```

`X-Forwarded-Proto: https` marks the session cookie `Secure`.
`X-Forwarded-For` is only trusted from loopback and private addresses (where
a proxy connects from), and only for rate-limiting sign-in attempts.

For uptime monitoring, `/api/health/live` reports whether Mizu itself is up;
`/api/health` also checks Sonarr and returns 503 while Sonarr is unreachable.

### Metrics

Mizu exposes Prometheus-style metrics at `/metrics` (public, like `/api/health`): request counts by method/route/status, sync cycle outcomes, episodes updated, AFL cache refresh outcomes, and Go runtime gauges. Point any scraper at it — no service discovery or extra config required.

## Architecture

```
cmd/server/            # HTTP server: one file per API area
  main.go              # Entry point, settings load, scheduler
  router.go            # Route table and middleware
  auth.go, settings.go, mappings.go, episodes.go, artwork.go,
  sonarr_series.go, afl.go, backup.go, health.go, system.go

internal/
  artwork/             # Artwork providers (AniList, TMDB, fanart.tv, Kitsu) and local storage
  auth/                # Password hashing and DB-backed sessions
  config/              # Runtime config with defaults (values live in SQLite)
  database/            # SQLite: goose migrations, sqlc queries (sqlcgen/)
  matcher/             # Jaro-Winkler fuzzy title matching
  metrics/             # Dependency-free Prometheus text exposition
  models/              # Shared types (Sonarr, AFL, mappings)
  notifier/            # ntfy push notifications
  scraper/             # AnimeFillerList HTML parser
  sonarr/              # Sonarr API v3 client with retries
  sync/                # Sync engine: scraper + matcher + Sonarr
  web/                 # Embeds the built SPA into the Go binary

web/                   # React SPA (Vite+, Astryx design system, TanStack Query)
  src/
    routes/            # Shows, ShowDetail, Activity, Settings, Onboarding, Login, AddAnime dialog
    components/        # Page sections, settings cards, add-anime wizard steps
    lib/               # API client, query hooks, schemas, themes
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

Endpoints are under `/api`, except `/metrics`. Once a password is set, everything except health, version, metrics and auth status/setup/login/logout needs a session.

| Method   | Path                                         | Description                                             |
| -------- | -------------------------------------------- | ------------------------------------------------------- |
| `GET`    | `/api/health`                                | Status incl. Sonarr (503 when Sonarr is unreachable)    |
| `GET`    | `/api/health/live`                           | Liveness: process and database only                     |
| `GET`    | `/api/version`                               | Version info                                            |
| `GET`    | `/metrics`                                   | Prometheus-style metrics                                |
| `GET`    | `/api/auth/status`                           | Whether a password is set and the caller is signed in   |
| `POST`   | `/api/auth/setup`                            | Set the initial password and sign in                    |
| `POST`   | `/api/auth/login`                            | Sign in                                                 |
| `POST`   | `/api/auth/logout`                           | Sign out                                                |
| `POST`   | `/api/auth/change-password`                  | Change the password (revokes all sessions)              |
| `POST`   | `/api/auth/remove-password`                  | Turn password protection off                            |
| `GET`    | `/api/stats`                                 | Dashboard statistics                                    |
| `GET`    | `/api/artwork/providers`                     | List available artwork providers                        |
| `GET`    | `/api/artwork/{id}/poster`                   | Serve poster artwork for a mapping                      |
| `GET`    | `/api/artwork/{id}/banner`                   | Serve banner artwork for a mapping                      |
| `GET`    | `/api/artwork/{id}/logo`                     | Serve logo artwork for a mapping                        |
| `POST`   | `/api/artwork/refresh-all`                   | Refresh artwork for all mappings (background)           |
| `GET`    | `/api/artwork/refresh-status`                | Whether an artwork refresh is running                   |
| `GET`    | `/api/mappings`                              | List all show mappings                                  |
| `POST`   | `/api/mappings`                              | Create mapping (auto-match or manual)                   |
| `GET`    | `/api/mappings/{id}`                         | Get a single mapping                                    |
| `PUT`    | `/api/mappings/{id}`                         | Update a mapping                                        |
| `DELETE` | `/api/mappings/{id}`                         | Delete a mapping                                        |
| `POST`   | `/api/mappings/{id}/artwork/refresh`         | Refresh artwork for a single mapping                    |
| `PUT`    | `/api/mappings/{id}/artwork/source`          | Change artwork source for a mapping                     |
| `GET`    | `/api/mappings/{id}/artwork/candidates`      | List candidate images from a source                     |
| `PUT`    | `/api/mappings/{id}/artwork/select`          | Store a picked candidate image                          |
| `GET`    | `/api/mappings/{id}/episodes`                | Get episodes for a mapping                              |
| `GET`    | `/api/mappings/{id}/episodes/sonarr`         | Get Sonarr episode data for a mapping                   |
| `PUT`    | `/api/mappings/{id}/episodes/{epId}/monitor` | Toggle episode monitor status                           |
| `POST`   | `/api/mappings/{id}/resync`                  | Resync a single mapping                                 |
| `POST`   | `/api/sync`                                  | Trigger manual sync                                     |
| `GET`    | `/api/logs`                                  | Get sync history                                        |
| `GET`    | `/api/settings`                              | Get current settings (API keys masked)                  |
| `PUT`    | `/api/settings`                              | Update settings                                         |
| `POST`   | `/api/settings/afl-cache/refresh`            | Trigger background AFL cache refresh                    |
| `GET`    | `/api/settings/afl-cache/status`             | Get AFL cache refresh status                            |
| `GET`    | `/api/sonarr/series`                         | List Sonarr series                                      |
| `GET`    | `/api/sonarr/series/{id}`                    | Get a single Sonarr series                              |
| `POST`   | `/api/sonarr/series`                         | Add a series to Sonarr                                  |
| `GET`    | `/api/sonarr/lookup`                         | Lookup series in Sonarr                                 |
| `GET`    | `/api/sonarr/add-options`                    | Get Sonarr add options (root folders, quality profiles) |
| `POST`   | `/api/sonarr/manual-match-pending`           | Mark a series as pending manual match                   |
| `POST`   | `/api/sonarr/test`                           | Test Sonarr connection                                  |
| `POST`   | `/api/ntfy/test`                             | Send test ntfy notification                             |
| `GET`    | `/api/afl/search?q=`                         | Search AnimeFillerList                                  |
| `GET`    | `/api/afl/suggest`                           | Get AFL suggestions                                     |
| `GET`    | `/api/backup/export`                         | Download config backup (includes API keys)              |
| `POST`   | `/api/backup/import`                         | Restore a config backup                                 |

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
go test ./...

# Tests with the race detector. -race needs CGO + a C compiler: MinGW gcc
# from PATH, or the default LLVM/clang install on Windows (Go 1.26+ handles
# MSVC-targeting clang). scripts/race-test.ps1 finds one automatically.
CGO_ENABLED=1 go test -race ./...

# Type-check
cd web && bunx tsc --noEmit

# Production build
cd web && bun run build
```

`bun run dev` starts both processes in one terminal:

- frontend Vite+ dev server on http://localhost:3001
- backend watcher (air) that restarts `go run ./cmd/server` whenever Go files change

The Vite+ dev server proxies /api requests to the Go backend on port 8978.

## License

MIT
