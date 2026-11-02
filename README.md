<p align="center">
  <img src="assets/mizu.svg" alt="Mizu" width="96">
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>

**Mizu** automatically unmonitors filler episodes in Sonarr using data from [AnimeFillerList](https://www.animefillerlist.com).

## Features

- Matches your Sonarr anime to AnimeFillerList and applies a rule per episode type (filler, mixed canon, anime canon)
- Per-show rules, artwork picker, and ntfy notifications (with per-show mute)
- Scheduled syncs, or run one on demand from the web UI
- Optional dashboard password, or run it behind your own auth proxy
- Single container, embedded SQLite

## Quick Start

```yaml
services:
  mizu:
    image: ghcr.io/OWNER/mizu:latest
    container_name: mizu
    ports:
      - "8978:8978"
    volumes:
      - ./config:/config
    environment:
      - TZ=UTC
    restart: unless-stopped
```

```bash
docker compose up -d
```

Open `http://localhost:8978` and enter your Sonarr URL and API key. Everything else is configured in the UI.

The container runs as uid 1000, so `./config` must be writable by it (`chown 1000:1000 config`).

## Build from source

```bash
cd web && bun install && bun run build && cd ..
cp -r web/dist/. internal/web/dist/
go build -o mizu ./cmd/server
```

## Notes

- Until you set a password, anyone who can reach the port has full access. Set one on first launch, or keep Mizu reachable only through your auth proxy.
- API keys are stored in plaintext in the database, and config backups include them.
- Mizu is not affiliated with Sonarr or AnimeFillerList.

## License

[MIT](LICENSE)
