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
    restart: unless-stopped
```

Mizu has no environment variables. Everything is set in the web UI and
stored in `/config/mizu.db`.

## Signing in

On first start, open Mizu and choose a password. Until one is set Mizu serves
nothing but that setup page, so do it right away.

**Behind a sign-in proxy** (tinyauth, Authelia, Authentik, ...), you can hand
sign-in to the proxy: open Mizu *through the proxy*, then in Settings → Security
→ Sign-in pick "Sign-in proxy" and list the proxy's address under trusted
proxy addresses. Mizu then answers only requests that come from those
addresses, so don't publish its port anywhere else.

**Trusted proxy addresses** also tell Mizu whose `X-Forwarded-For` and
`X-Forwarded-Proto` headers to believe (for login rate limiting and HTTPS
detection). A proxy on the same host (loopback) is always trusted.

**Forgot your password?** Stop Mizu and clear it, then set a new one in the
browser:

```sh
sqlite3 config/mizu.db "UPDATE app_settings SET value = '' WHERE key = 'auth_password_hash'; UPDATE app_settings SET value = 'password' WHERE key = 'auth_mode'; DELETE FROM auth_sessions;"
```

## Metrics

`/metrics` serves Prometheus metrics to signed-in users. For a scraper, use
HTTP basic auth with the dashboard password (any username):

```yaml
scrape_configs:
  - job_name: mizu
    static_configs:
      - targets: ["mizu:8978"]
    basic_auth:
      username: prometheus
      password: <your Mizu password>
```

## Backups

Settings → Backup → Export downloads your shows and settings. API keys and
tokens are left out unless you switch on "Include API keys and tokens", which
asks for your password. Keep a file with keys private.

## Running without Docker

`mizu` listens on `:8978` and keeps its database in the working directory.
Pass `-addr 127.0.0.1:8978` to listen on loopback only (e.g. behind a reverse
proxy on the same host).

## License

[MIT](LICENSE)
