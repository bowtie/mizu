# Changelog

## [Unreleased]

### Sonarr webhook

- Sonarr can call Mizu the moment it grabs an episode or adds a series
  (Settings → Sonarr webhook has the URL and password for Sonarr's
  Connect → Webhook). A mapped show is resynced right away instead of at
  the next scheduled sync, waiting for a running one to finish.
- A grabbed episode that was monitored and that the resync unmonitors is
  cancelled: the download is removed from the queue and the download
  client, without blocklisting. Episodes grabbed by hand while unmonitored
  are left alone.
- Webhook runs show in Activity as "Sonarr webhook".
- The webhook checks its own password (Basic auth or `X-Api-Key`) in every
  sign-in mode, so it also answers under a container name with sign-in off.
  The password isn't part of config exports.

### Filler cleanup

- A show's toolbar has a Delete filler files button. It lists the
  downloaded files that only hold episodes the show's rules skip, with
  their size, and deletes the picked ones from disk through Sonarr.
- Files are picked by default unless one of their episodes was monitored
  again. A file shared with a kept or unclassified episode (a double
  episode, say) is never offered.
- Deleted episodes are unmonitored so Sonarr doesn't download them again.
  Each cleanup shows in Activity as "Filler cleanup" with the space freed.

## [0.4.0] - 2026-11-03

Security pass on 0.3.0. Every setting lives in the database, sign-in is a
choice made at first run, and stored secrets stay on the server.

### Security

- Sign-in is the welcome flow's third step: set a password, or skip it.
  Until that choice is made a fresh install answers without sign-in, so
  Sonarr can be connected first (the first visitor could already turn
  sign-in off, so this opens nothing new). Settings → Security switches
  between the Mizu password, a sign-in proxy and no sign-in. Turning sign-in
  off needs the current password and revokes every session; turning it back
  on sets a new password and signs the caller in. Two simultaneous first-run
  setups can't both succeed.
- With sign-in off, Mizu only answers requests addressed to an IP address,
  localhost or a listed hostname (Settings → Security → Allowed hostnames),
  so a DNS-rebinding page can't drive it through a LAN browser. The hostname
  setup was opened under stays allowed, and turning sign-in off in Settings
  keeps answering to the hostname it was done from.
- Sign-in proxy mode answers only requests from the listed trusted proxy
  addresses, so the proxy can't be bypassed and DNS rebinding gets nowhere.
- Settings → Security shows only what the chosen sign-in mode needs. A
  sign-in proxy's address is filled in from the request it forwarded, and
  password sign-in tucks trusted proxies under "Behind a reverse proxy?".
  `/api/auth/status` reports `mode` instead of `external` / `open`.
- No environment variables: `MIZU_PASSWORD` and `MIZU_ADDR` are gone. Mizu
  listens on `:8978` (or `-addr`), and all settings, including sign-in mode,
  trusted proxies and allowed hostnames, live in the database.
- Stored API keys never leave the server unasked: changing the Sonarr or ntfy
  URL requires the key or token again (it was silently sent to the new URL),
  config imports clear a credential whose URL changed, and exports leave keys
  out unless requested with the password (or without one while sign-in is
  off).
- `X-Forwarded-For` / `X-Forwarded-Proto` are trusted only from configured
  proxies (and loopback). Login rate limiting counts only failures, can't be
  used to lock the owner out, and keeps its memory bounded. Password checks on
  every endpoint share one budget.
- Outbound requests refuse link-local, cloud-metadata, multicast and
  unspecified addresses. Sonarr and ntfy URLs can't carry credentials, query
  strings or fragments, and redirects to another host are refused (Go would
  forward the Sonarr API key). Sonarr error bodies are no longer echoed back.
- Security headers: a Content Security Policy (inline scripts allowed by hash),
  `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy`, and HSTS over HTTPS.
  Session cookies are `Secure` whenever the browser used HTTPS, even behind a
  proxy that doesn't send `X-Forwarded-Proto`.
- Mutating API requests must be JSON (multipart only for artwork uploads), on
  top of the same-origin check.
- `/metrics` needs sign-in (or basic auth with the dashboard password),
  `/api/version` needs sign-in, and `/api/health` tells strangers only up or
  down.
- Internal errors are logged, not returned. The fanart.tv key no longer
  appears in logs. A bogus session cookie no longer triggers a database write;
  expired sessions are swept hourly.
- `mizu.db` (and its WAL/SHM files) are created owner-only (`0600`), and
  existing files are tightened.
- Artwork thumbnails are size-checked before decoding.
- The offline API cache is cleared whenever a session ends.
- Releases pin actions to commit SHAs and base images to digests, build
  without the shared CI cache, and publish signed images with provenance and
  an SBOM. Dependabot keeps the pins current.

### Fixes

- The AnimeFillerList show list refreshes on its own, so new titles reach
  the add dialog and new-title alerts. Settings → Sync → AFL refresh picks
  how often: daily, weekly (default), monthly or manual only. An empty list
  is always filled. Before, only the Settings button refreshed it: the
  30-day refresh hung off an auto-match path the dashboard never used.
- The sign-in page lives at a plain `/login`; the page to return to is
  remembered for the browser session instead of riding along as `?from=`.
- Custom artwork uploads over 1 MiB are accepted again, up to the advertised
  10 MB (the global request body cap rejected them with a 413).
- Settings reads "Never refreshed" for an AnimeFillerList cache that was never
  refreshed, instead of "2,027 years ago".
- Episode monitor toggles show the episode's state (an open eye when
  monitored) and keep a visible button when unmonitored.
- The last episode in a season no longer draws a thick divider under it.
- Season headers highlight the whole card on hover instead of a strip inside
  it.
- The sign-out button is hidden while sign-in is off.

### Engineering

- Unreachable Go functions and unused web exports and types removed.
- A GitHub social preview image (`assets/social-preview.png`) with its HTML
  source.

## [0.3.0] - 2026-11-02

First public release. Covers everything since 0.2.0.

### Security

- Selectable auth modes: Mizu's own password, or none when an auth proxy in
  front handles sign-in. Stored API keys are no longer sent to the browser,
  and `maskKey` reveals no characters of a key.
- Works behind reverse proxies and auth proxies: `X-Forwarded-Host` and
  `X-Forwarded-Proto` handling for the same-origin guard and `Secure`
  cookies, and `X-Forwarded-For` trusted only from private addresses.

### Notifications

- Per-show mute (bell in the show toolbar). Muted shows are left out of sync
  pushes, and a run that only touched muted shows sends nothing. Mute
  round-trips through config backup and import.
- New-title alerts: shows that newly appear on AnimeFillerList are logged to
  activity and can send one ntfy push (`ntfy_notify_new_titles`, off by
  default).

### Artwork

- Redesigned artwork dialog with slot tabs, a source switcher, and a footer
  with automatic source, Refresh and Done.
- Near-duplicate candidates are merged by perceptual hash.
- Custom uploads per slot (JPG, PNG, GIF, WebP, up to 10 MB) that survive
  refreshes and a switch to Sonarr.
- Artwork URLs are versioned so changed images are not served stale.

### Frontend

- First-run welcome flow, error retry, settings save bar, and accessibility
  and small-screen fixes.
- TanStack Query is the only client cache. Routes and dialogs are lazy-loaded
  with prefetch, service worker offline caching, and a self-hosted font.
- `ShowDetailPage`, `SettingsPage` and `AddAnimeDialog` split into components.

### Operations

- Catch-up sync at startup and a last-synced indicator.
- Go modules, web dependencies and the toolchain bumped (Go 1.27).
- CI for vet, lint, race tests, web checks and the Docker build, plus a
  tag-triggered release workflow that publishes a multi-arch image to GHCR.
- MIT license and the Figtree OFL notice.

## [0.2.0] - 2026-08-24

Security hardening and reliability pass: dashboard authentication, scraper
and Sonarr failure-mode fixes, and operational tooling.

### Security

- Add single-password dashboard authentication (*arr-style): bcrypt-hashed
  password configured on first run (or seeded via `MIZU_PASSWORD`), DB-backed
  sessions with HttpOnly SameSite cookies, login/logout, and change-password
  in Settings. Auth is disabled until a password exists; API keys are no
  longer readable by anonymous callers.
- Reject cross-origin mutating requests (same-origin guard) and ship a
  `.dockerignore` so databases holding plaintext keys never enter image
  layers.

### Reliability

- Scraper: a zero-count show-list parse or missing episode table is now an
  error instead of silently wiping cached coverage; AFL outages fall back to
  the stale show cache; response bodies are capped at 8 MiB.
- Sonarr client: GET/PUT retry transient failures (transport errors, 429,
  5xx) with capped backoff and `Retry-After` support; POST retries only on
  dial-level failures to avoid double-adds. Response reads are size-capped.
- Sync trigger/resync handlers claim the engine's run slot before answering,
  so concurrent triggers get a real 409 instead of a no-op 202.
- AFL cache refresh runs in the background (202 + status endpoint) so long
  scrapes survive client disconnects and the server write timeout.

### Operations

- `/api/health` caches the Sonarr probe (60s TTL) and returns 503 when
  degraded, making the Docker healthcheck meaningful.
- New `/metrics` endpoint exposing Prometheus-style counters/gauges
  (requests per method/route/status, sync cycles, AFL refreshes, build
  info, runtime) — dependency-free implementation.
- Sync logs are pruned to the newest 200; startup marks logs stuck in
  `running` by a previous shutdown as failed.
- Runtime base image bumped from EOL alpine 3.19 to 3.24.

### Frontend

- Login/setup page with route guarding and automatic redirect on session
  expiry; sign-out button in the nav; security card in Settings for
  password change and sign-out.
- Settings page polls AFL cache refresh status and reflects background
  completion or failure.

## [0.1.0] - 2026-08-18

First release candidate. This is a broad concurrency and correctness hardening
pass on top of the existing Mizu codebase.

### Reliability — race fixes

- Guard artwork fetch write-backs against resurrecting files after a mapping is
  deleted or its artwork source is switched to Sonarr.
- Guard sync write-backs so a mid-sync user edit (episode toggle, watermark, or
  settings change) is not clobbered by a stale sync snapshot (per-mapping epoch).
- Make newer per-show artwork intent win over an older refresh-all pass
  (per-mapping intent counter).
- Guard prefetch and episode ETag/data cache write-backs against an older
  response overwriting newer data (per-key generation).
- Coalesce concurrent AFL show-list scrapes through `singleflight` so a manual
  refresh and the stale-cache fallback never double-scrape.
- Guard the theme-selection and artwork-refetch async patches against
  superseded selections and unmounted components.

### Frontend UX

- Add a top-level React error boundary with a reload fallback.
- `ActivityPage` and `ShowsPage` keep polling until a sync or artwork refresh
  reaches a terminal state (fast phase + slower watchdog cadence, with
  focus/visibility recovery), instead of freezing at an attempt cap or on a
  transient error.
- Clear stale local artwork files when a refresh resolves to the Sonarr source,
  so the Sonarr image actually takes effect.

### Performance

- Batch settings persistence into a single transaction.
- Persist the `overview` column in the mapping upsert, removing per-field
  workaround writes.

### Engineering

- Extract shared helpers (artwork refetch delay, vitest DOM setup) and add
  regression tests for every race fix above.
- Normalize line endings to LF and remove dead code so `golangci-lint` passes
  clean.
- Format the frontend with the project's `oxfmt` (`vp check` is clean).
- Resolve the frontend typecheck errors so `tsc -b && vp build` succeeds end to
  end.
