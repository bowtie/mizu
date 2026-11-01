# Bug Fix Log

## Bug Audit Summary

**39 bugs fixed across four audit passes.** All Go tests pass, all frontend tests pass (32/32), both build clean.

### Go Backend (7 bugs)
| # | File | Line(s) | Severity | Status |
|---|------|---------|----------|--------|
| G1 | `cmd/server/router.go` | 900-924 | Medium | ✅ Fixed |
| G2 | `cmd/server/router.go` | 1796-1830 | Medium | ✅ Fixed |
| G3 | `internal/models/models.go` | 114 | Low | ✅ Fixed |
| G4 | `cmd/server/router.go` | 1332-1335 | Low | ✅ Fixed |
| G5 | `cmd/server/router.go` | 422-455 | Low | ✅ Fixed |
| G6 | `cmd/server/router.go` | 1647 vs 1888 | Low | ⏭️ By design (lookup needs full fields) |
| G7 | `internal/database/database.go` | 360-374 | Low | ⏭️ Non-issue (aggregate query always returns a row) |

### Frontend (11 bugs)
| # | File | Line(s) | Severity | Status |
|---|------|---------|----------|--------|
| F1 | `ShowDetailPage.tsx` | 728-729 | Critical | ✅ Fixed |
| F2 | `ShowDetailPage.tsx` | 618-752 | Medium | ⏭️ Design tradeoff — deferred |
| F3 | `ActivityPage.tsx` | 156-159 | Medium | ✅ Fixed |
| F4 | `ShowsPage.tsx` | 336-337 | Medium | ✅ Fixed |
| F5 | `ShowDetailPage.tsx` | 534-538 | Medium | ✅ Fixed |
| F6 | `TrackingRuleCard.tsx` | 6 | Medium | ✅ Fixed |
| F7 | `ShowDetailPage.tsx` | 1276 | Low-Med | ✅ No fix needed (`info` is a valid Badge variant) |
| F8 | `ShowDetailPage.tsx` | 226-243 | Low | ✅ Fixed |
| F9 | `ShowDetailPage.tsx` | 389 | Low | ✅ Fixed |
| F10 | `VirtualizedEpisodeList.tsx` | 156 | Low | ✅ Fixed |
| F11 | `index.css` | 469 | Low | ✅ Fixed |

---

## Fix Log

### F1 — Episode fetch error silently swallowed (Critical)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:728`
**Fix:** Removed the `if (!showApplied)` gate from the `getEpisodes` catch handler. Episode fetch errors now always show a notification instead of being swallowed when the show fetch succeeds.

### G1 — Episode monitor not saved to DB (Medium)
**File:** `cmd/server/router.go:900-924`, `internal/database/database.go`
**Fix:** Added `DB.UpdateEpisodeMonitor()` wrapper that calls the existing sqlc `UpdateEpisodeMonitor` query. The handler now calls `db.UpdateEpisodeMonitor()` after the Sonarr update succeeds, so the change persists in SQLite beyond cache invalidation.

### G2 — Inconsistent series-by-ID response shape (Medium)
**File:** `cmd/server/router.go:1796-1830`
**Fix:** Cache hit path now calls `cached.Public()` to match the cache miss path which already calls `series.Public()`. Both paths now return the same `SonarrSeriesPublic` shape.

### F4 — Stale settings in Add Anime dialog (Medium)
**File:** `web-astryx/src/routes/ShowsPage.tsx:336`
**Fix:** `refreshSettings` now uses `queryClient.fetchQuery()` instead of `invalidateQueries()` + returning stale `settingsQuery.data`. The dialog now waits for fresh settings before applying default tracking rules.

### F3 — Duplicate polling timers in StrictMode (Medium)
**File:** `web-astryx/src/routes/ActivityPage.tsx:156`
**Fix:** Moved the `setTimeout` call out of the `setLogs` state updater. Added a `logsRef` that tracks the latest logs so the polling logic can check for running status without scheduling side effects inside a reducer.

### F5 — Mobile filter count mismatch (Medium)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:534`
**Fix:** Added `isMobile` prop to `EpisodeListSection`. The "Manga" filter label now shows `epStats.canon + epStats.anime` on mobile (matching the actual filtered episodes) and `epStats.canon` on desktop.

### F6 — Static cuelume import (Medium)
**File:** `web-astryx/src/components/TrackingRuleCard.tsx:6`
**Fix:** Replaced the static `import { play } from "cuelume"` with a lazy dynamic import + null guard, matching the defensive pattern already used in `store.ts`. Both call sites now check `if (cuelumePlay)` before calling.

### G3 — CompletedAt zero time serialization (Low)
**File:** `internal/models/models.go:114`, `internal/database/database.go:181`, `cmd/server/router.go:168-176`
**Fix:** Changed `SyncLog.CompletedAt` from `time.Time` to `*time.Time` with `omitempty`. In-progress syncs now serialize as `"completed_at": null` instead of `"0001-01-01T00:00:00Z"`. Updated the DB conversion and the health check handler to handle the nil pointer.

### F8 — Stale openedSeasons on filter change (Low)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:226-245`
**Fix:** The `useEffect` that auto-opens the last season now also cleans up stale seasons that no longer exist in the filtered results. Empty filter results clear the set entirely.

### F9/F10 — Episode key collisions (Low)
**Files:** `web-astryx/src/routes/ShowDetailPage.tsx:391`, `web-astryx/src/components/VirtualizedEpisodeList.tsx:156`
**Fix:** Added `ep.sonarr_episode_id` to the React key in both the standard and virtualized episode lists, preventing duplicate keys when episodes share the same display number.

### F11 — CSS typo `margininline` (Low)
**File:** `web-astryx/src/index.css:469`
**Fix:** Changed `margininline` to `margin-inline`.

### F7 — Badge variant="info" validity (Low-Med)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:1276`
**Result:** Verified against `@astryxdesign/core` — `"info"` is a valid Badge variant. No fix needed.

---

## Second Round Fixes

### G4 — AFL cache refresh "partial" status for successful refreshes (Low)
**File:** `cmd/server/router.go:1333-1335`
**Fix:** Removed the `if len(newSlugs) > 0 { status = "partial" }` branch. A successful refresh that detects new shows is now logged as `"success"` — the new items are already logged individually with `"new_in_cache"` status. The `"partial"` status is now reserved for actual partial failures, matching its UI presentation (yellow warning triangle).

### G5 — Auto-match blocking on empty AFL cache (Low)
**File:** `cmd/server/router.go:422-427`
**Fix:** Added a pre-check before calling `AutoMatchSeries`. If the AFL cache is empty (count == 0), the handler returns a 409 Conflict with a clear message: "AFL cache is empty — refresh the AFL cache before using auto-match". This prevents the handler from synchronously scraping the entire AFL website within the HTTP timeout.

---

## Second-Pass Fixes (Go Backend)

### G2-Bug2 — DB stale monitored status after sync (Medium)
**File:** `internal/sync/sync.go:516`
**Fix:** After building `toMonitor`/`toUnmonitor` lists, update `dbEpisodes[i].Monitored` to reflect post-sync status before calling `ReplaceEpisodesForMapping`. The DB now matches Sonarr's actual state immediately after sync, without waiting for the next sync cycle.

### G2-Bug4 — Episode cache not invalidated after sync/resync (Medium)
**File:** `cmd/server/router.go:942-979`
**Fix:** Added `epCache` parameter to `handleResyncMapping` and `handleTriggerSync`. After background sync/resync completes, `epCache.invalidate(id)` (resync) or `epCache.invalidateAll()` (full sync) is called. Added `invalidateAll()` method to `episodeCache`.

### G2-Bug1 — Artwork FetchAndStore race condition (Medium)
**File:** `internal/artwork/manager.go:22-40, 112-114`
**Fix:** Added per-mapping mutex via `mapLocks map[int64]*sync.Mutex`. `FetchAndStore` now acquires `lockMapping(mappingID).Lock()` before any file operations, preventing concurrent calls for the same mapping from corrupting each other's writes.

### G2-Bug9 — Cannot clear API keys via settings (Low)
**File:** `cmd/server/router.go:1148, 1248, 1251`
**Fix:** Removed the `!= ""` guard from API key update conditions. Now `if update.SonarrAPIKey != nil` (same for TMDB and Fanart) allows setting the key to an empty string to clear it, matching the ntfy token behavior.

### G2-Bug5 — Missing index on (mapping_id, sonarr_episode_id) (Low)
**File:** `internal/database/migrations/001_initial_schema.sql:106-108`
**Fix:** Added `idx_episode_info_mapping_sonarr_id` index on `(mapping_id, sonarr_episode_id)` to speed up `UpdateEpisodeMonitor` queries.

---

## Second-Pass Fixes (Frontend)

### F2-Bug1 — SW caches API responses containing full API keys (Critical)
**File:** `web-astryx/public/sw.js:73-89`
**Fix:** Excluded `/api/settings` and `/api/bootstrap` from the stale-while-revalidate cache strategy. These endpoints now use network-first (with cache fallback only on network failure), preventing API keys from being stored in Cache Storage.

### F2-Bug2 — RETURN_TO_SERIES_STEP doesn't clear selectedSonarr (Medium)
**File:** `web-astryx/src/routes/addWizardHelpers.ts:136-147`
**Fix:** Added `selectedSonarr: null` to the `RETURN_TO_SERIES_STEP` reducer case, preventing the stale "Continue" button from appearing after "Add another".

### F2-Bug3 — Unbounded episodeETags/episodeDataCache Maps (Medium)
**File:** `web-astryx/src/lib/api.ts:189-193`
**Fix:** `clearCachedShowDetail` now also deletes entries from `episodeETags` and `episodeDataCache`, preventing unbounded memory growth in long sessions.

### F2-Bug5 — useMarkManualMatchPending doesn't invalidate queries (Medium)
**File:** `web-astryx/src/lib/queries.ts:346-348`
**Fix:** Added `onSuccess` handler that invalidates `["sonarrSeries"]` queries so the `manualMatchPending` flag is fresh.

### F2-Bug6 — useRefreshAllArtwork doesn't invalidate shows query (Medium)
**File:** `web-astryx/src/lib/queries.ts:291-293`
**Fix:** Added `onSuccess` handler that invalidates `shows` and `bootstrap` queries so stale artwork paths are refreshed.

---

## Third-Pass Fixes

### G2-Bug3 — UpsertMapping not persisting overview on creation (Medium)
**Files:** `cmd/server/router.go:543-552`, `internal/sync/sync.go:956-964`
**Fix:** After `UpsertMapping`, call `UpdateOverview` separately when an overview is available. The sqlc-generated `upsertMapping` query doesn't include the `overview` column, so it was silently dropped on both manual and auto-match creation paths. The overview is now persisted immediately instead of waiting for the next sync's `processSeries` to backfill it.

---

## Deferred Items (Second Pass) — Final Assessment

**Fixed in third pass:**
- **G2-Bug10** — N+1 query for sync log items. **Fixed** — replaced per-log-ID loop with a single batched `WHERE sync_log_id IN (...)` query in `getSyncLogItemsBatch`, used by both `GetSyncLogsPage` and `GetSyncLogItems`.
- **F2-Bug9** — No `beforeunload` warning on settings page. **Fixed** — added `useEffect` on `SettingsPage.tsx` that registers a `beforeunload` listener when `settingsDirty` is true and cleans it up otherwise.

**Remaining deferred (accepted as-is):**
- **G2-Bug6** — `handleResyncMapping` TOCTOU race on `IsRunning()`. Low impact — the CAS inside `RunResync` handles it correctly, just the error response is lost.
- **G2-Bug7** — Notifier has no retry logic. Low impact — notifications are best-effort.
- **G2-Bug8** — API keys sent unmasked in JSON responses. This is by design for the settings page to populate form fields. The SW fix prevents caching.
- **G2-Bug11** — `FindMatches` ignores context cancellation. Low impact — matching is a pure CPU computation with no I/O, completing in microseconds.
- **F2-Bug4** — Stale closure in `loadSuggestedMatches`. The `suggestReqId` guard prevents applying wrong results; only the manual search box visibility can be briefly wrong.
- **F2-Bug7** — SW serves stale API data on offline with no expiry. Design tradeoff — offline functionality requires serving cached data.
- **F2-Bug8** — Stale closure in `filterFallbackAflResults`. Mostly masked by timing; edge case.
- **F2-Bug10** — `quality_profile_id: 0` edge case. Already handled — `handleAddSonarrSeries` validates `qualityProfileID <= 0` and returns a 400 error.
- **F2** — `useEffect` deps exclude `storeShow`/`prefetchedDetail`. Design tradeoff — the current behavior prefers network data over store data, which is generally correct.

---

## Third-Pass Bug Fixes

### Go Backend (4 bugs)

#### G3-Bug3 — Partial config mutation on validation failure (Medium)
**File:** `cmd/server/router.go:1124-1265`
**Fix:** Restructured `handleUpdateSettings` to validate all fields before mutating any config. Previously, early fields (SonarrURL, SonarrAPIKey) were mutated before later validation checks (filler action, monitor mode, ntfy config), causing the in-memory config to diverge from the DB on validation failure. All validation now runs first, then all mutations apply atomically under the write lock.

#### G3-Bug6 — `handleSetArtworkSource` rejects "auto" (Medium)
**File:** `cmd/server/router.go:2285`
**Fix:** Added `"auto"` to the valid source values in `handleSetArtworkSource`. Other endpoints (`handleRefreshArtwork`, `handleUpdateSettings`) already accepted `"auto"`, but this endpoint rejected it with "unknown artwork source: auto", making it impossible to set a per-show override to auto mode.

#### G3-Bug13 — DB/Sonarr desync on bulk monitor failure (Medium)
**File:** `internal/sync/sync.go:536-563`
**Fix:** Previously, the DB was updated with post-sync monitored status before the Sonarr bulk API call. If the Sonarr call failed, the DB had the wrong state. Now the DB is reconciled after the Sonarr call — if a bulk operation fails, the affected episodes' monitored status is reverted in the DB to match Sonarr's actual state, and the corrected state is re-persisted.

#### G3-Bug20 — Negative computed absolute episode number (Medium)
**File:** `internal/sync/sync.go:446-460`
**Fix:** Added a guard to only use the computed absolute episode number when it's greater than the last known absolute number. Out-of-order episodes (recaps, rebroadcasts) could produce negative or incorrect values, causing episodes to be misclassified as below the watermark and incorrectly unmonitored.

### Frontend (9 bugs)

#### F3-Bug1 — Stale show data when navigating between shows (High)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:763-779`
**Fix:** The store-adoption effect guarded with `if (storeShow && !show)`, which failed when navigating between two shows both in the TanStack Query store — `show` was still non-null from the previous show. Changed guard to `show?.id !== storeShow.id` so the state updates to the new show.

#### F3-Bug2 — Race condition on show navigation (High)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:626-758`
**Fix:** Added a `cancelled` flag in the main `useEffect` cleanup. Previously, in-flight `getShow`/`getEpisodes` promises from a previous show could resolve after navigation and overwrite state with stale data. All `.then()` callbacks now check the flag before calling state setters.

#### F3-Bug3 — Hero image error state not reset after artwork refresh (Medium)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:916-979`
**Fix:** Added `setHeroImgError(false)` and `setHeroFallbackFailed(false)` alongside every `setArtworkVersion` call in `handleRefreshArtwork` and `handleArtworkSourceChange`. Previously, if the local artwork had errored (causing the hero to fall back to Sonarr images), the hero would never try the new local artwork after a refresh.

#### F3-Bug4 — "Saved" flash shows even on save failure (Medium)
**File:** `web-astryx/src/routes/SettingsPage.tsx:357-364`
**Fix:** Moved `setSavedFlash(true)` and the timeout inside the `try` block, after the successful `mutateAsync` call. Previously it was after the catch block, so the green "Saved" indicator appeared even when the save failed.

#### F3-Bug5 — "Load more" button can be double-clicked (Medium)
**File:** `web-astryx/src/routes/ActivityPage.tsx:432-438`
**Fix:** Added `isDisabled={loadingMore}` to the "Load more" button. Without this, multiple clicks before the first request completed would fetch the same page twice and append duplicate log entries.

#### F3-Bug6 — Uncancelled requestAnimationFrame on navigation (Medium)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:626-758`
**Fix:** All `requestAnimationFrame` calls now store their IDs in an array and are cancelled in the effect cleanup via `cancelAnimationFrame`. Previously, a rAF from a previous show's effect could fire after navigation and incorrectly set `pageSettled` to true before the new show's data loaded.

#### F3-Bug7 — Stale episodes not cleared on navigation (Medium)
**File:** `web-astryx/src/routes/ShowDetailPage.tsx:626-758`
**Fix:** Added `setEpisodes([])`, `setEpStats({...zeros})`, and `setMaxEpisode(0)` at the top of the main effect. Previously, the old show's episodes remained visible (with old season groupings and stats) until the new show's episodes loaded.

#### F3-Bug8 — 304 response with empty cache causes error (Low)
**File:** `web-astryx/src/lib/api.ts:341-348`
**Fix:** When the server returns 304 but `episodeDataCache` doesn't have the key (e.g., cache was cleared), the ETag is now deleted and the request is retried without conditional headers. Previously, the 304 fell through to the error path, and since 304 responses have no body, `res.json()` failed with a confusing "Not Modified" error.

#### F3-Bug9 — Theme CSS load failure not retryable (Low)
**File:** `web-astryx/src/lib/themes.ts:82-90`
**Fix:** Moved `loadedThemeCSS.add(themeId)` to after the `await import(...)` succeeds. Previously, the themeId was added before the import, so a failed import (network error) would permanently prevent the theme CSS from loading on retry.
