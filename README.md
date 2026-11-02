<p align="center">
  <img src="assets/mizu.svg" alt="Mizu" width="96">
</p>

<p align="center">
  <b>Mizu</b> automatically unmonitors filler episodes in Sonarr using data from <a href="https://www.animefillerlist.com">AnimeFillerList</a>.
</p>

## Features

- Matches your Sonarr anime to AnimeFillerList and applies a rule per episode type (filler, mixed canon, anime canon)
- Per-show rules, artwork picker, and ntfy notifications
- Scheduled syncs, or run one on demand from the web UI

## Quick Start

```yaml
services:
  mizu:
    image: ghcr.io/bowtie/mizu:latest
    container_name: mizu
    ports:
      - "8978:8978"
    volumes:
      - ./config:/config
    environment:
      - TZ=UTC
    restart: unless-stopped
```

## License

[MIT](LICENSE)
