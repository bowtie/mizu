# Changelog

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
