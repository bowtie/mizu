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

- A show's toolbar has a Delete filler files button, shown only when
  there is something to delete. It lists the
  downloaded files that only hold episodes the show's rules skip, with
  their size, and deletes the picked ones from disk through Sonarr.
- Files are picked by default unless one of their episodes was monitored
  again. A file shared with a kept or unclassified episode (a double
  episode, say) is never offered.
- Deleted episodes are unmonitored so Sonarr doesn't download them again.
  Each cleanup shows in Activity as "Filler cleanup" with the space freed.

### Unlisted episodes

- New setting, Settings → Sync → Unlisted episodes: new episodes that
  AnimeFillerList hasn't listed yet can stay unmonitored for up to 3, 7 or
  14 days, so Sonarr doesn't grab a filler before it's listed. Off by
  default, which keeps monitoring them like canon.
- Once AnimeFillerList lists a held episode, the show's rules decide as
  usual. If it's still unlisted when the time is up, it's monitored again.
  Turning the setting off, or monitoring a held episode by hand, releases
  it too.
- Only episodes airing within the hold window (or without an air date) are
  held. Downloaded episodes and specials never are. Activity shows
  "holding N for AnimeFillerList" while a show has held episodes.
- "Until AFL lists them" holds new episodes with no time limit. It starts
  holding only episodes from the last 14 days, so switching it on doesn't
  hold years of episodes AFL never covered.
- Held episodes show a "Waiting" badge in the show's episode list.

### Matching

- Sonarr episodes are lined up with AnimeFillerList's numbering by title,
  episode by episode. TVDB often counts episodes AFL doesn't: specials
  filed under absolute numbers (Cardcaptor Sakura's Clear Card is 77-98 in
  Sonarr, 72-93 on AFL) or banned episodes kept in place (Pokémon drifts
  one further with each). Matching by number gave episodes a neighbour's
  type: 118 Pokémon episodes had the wrong one. Episodes whose titles match
  AFL exactly anchor the numbering, the rest follow the nearest anchor,
  and an episode AFL leaves out is left unclassified (so monitored).
- AFL's air dates are read too. A season whose titles are translated too
  differently to settle it can anchor by date instead (within a day, to
  allow for time zones). Dates are sometimes a week off where a broadcast
  moved, so only the season's common shift counts, those dates must cover
  at least half the season, and titles always win.
- Add anime no longer suggests the AFL list of a different series with a
  similar name: an AFL title naming another year (Urusei Yatsura (1981)
  for the 2022 remake) isn't a match, and a Sonarr title with a word the
  AFL title lacks (Gundam Wing, Prince of Tennis II, Fullmetal Alchemist:
  Brotherhood) drops below the auto-match threshold. These used to match
  at over 90%.
- Specials that match an AFL episode are listed with the show's episodes
  (Cardcaptor Sakura's Clear Card prologue, AFL episode 71). They were
  already synced; only the list left them out.

- AFL shows whose address has an accent, apostrophe or symbol can be
  added and synced. AFL percent-encodes those (pok%C3%A9mon-advanced-
  generation, kuroko%E2%80%99s-basketball, ranma-%C2%BD) and Mizu refused
  them as invalid, so 15 shows, among them Kuroko's Basketball, Hell's
  Paradise, Ranma ½ and two Pokémon series, couldn't be fetched at all.

- Add anime matches AFL titles that carry a bracketed alternate name:
  "Fire Force (Enen no Shouboutai)" now matches Sonarr's "Fire Force" (it
  scored 0.31), likewise Sailor Moon, Sgt. Frog, Kamisama Kiss and others.
  Hunter x Hunter still matches AFL's "Hunter × Hunter".
- Titles written with "×" match ones with "x" (Hunter × Hunter, High
  School D×D), AFL's slug counts as another name for a show (AFL's
  "Magi" is magi-labyrinth-magic) and as a source of its year, and small
  connecting words ("of", "no") don't mark a title as another series.
- A season is no longer moved onto AFL's numbers by dates alone when its
  titles fit better unmoved: Saint Seiya's Asgard arc, whose TVDB dates run
  seven weeks behind AFL's, had been shifted onto the wrong episodes.
- An AFL page with no episodes listed yet says so instead of suggesting
  AFL's layout changed.

### Engineering

- `go run ./cmd/audit -db mizu.db` reports, per mapped show, where Sonarr
  and AFL disagree under Mizu's matching; `-catalogue` does the same for
  every AFL show with filler, using TVDB's episodes from Sonarr's metadata
  service; `-numbers <slug>` previews a show's numbering. It only reads.

### Show page

- Show pages have readable URLs from the Sonarr title
  (/shows/cardcaptor-sakura). Old /shows/67 links still work and switch to
  the slug; titles that would clash get the show's id appended.

- Tracking rule toggles save at once, like the mute bell and artwork
  picks already did. Auto/Manual and the episode marker wait for a Save
  that shows on the toolbar only while one of them has a change (on
  phones, the ⋯ button turns into a ✓). The "Unsaved changes" badge and
  the always-there Save button are gone.

### Phones

- Show page: the actions bar tucks behind the banner on touch screens;
  tap the banner (the chevron hints at it) to slide it out and again to
  put it away. On phones it is one row: the episode stepper, Auto, and a
  ⋯ menu with Artwork, Mute notifications, Delete filler files and Remove;
  the stepper matches the buttons' height.
- Navigation is a bottom tab bar (Library, Activity, Settings) instead of
  a hamburger drawer; the Settings save bar and toasts sit above it.
- Episode rows put the air date under the title instead of squeezing
  titles to a few characters, and badges show whole. Episode and Activity
  filters are dropdowns instead of tabs that ran off screen.
- Tracking rules are compact stacked rows.
- Library, Activity and Settings fit more on screen: icon buttons for Add
  anime, sort and Sync now, search sharing a row with its controls,
  shorter banner cards, two posters a row, and single buttons beside
  their labels in Settings.
- Add anime, the artwork picker and the filler files dialog open
  fullscreen with even padding; the Add anime list uses the spare height,
  and switches stay beside their labels.
- The episode number field fits its number and centers it.

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
