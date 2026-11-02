<p align="center">
  <img src="assets/mizu.svg" alt="Mizu" width="96">
</p>

<p align="center">
  <b>Mizu</b> automatically unmonitors filler episodes in Sonarr using data from <a href="https://www.animefillerlist.com">AnimeFillerList</a>.
</p>

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
