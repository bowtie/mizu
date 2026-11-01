# Changelog

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
