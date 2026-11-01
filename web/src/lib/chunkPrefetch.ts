/**
 * Prefetch lazy-loaded route chunks on user intent (hover/focus).
 *
 * Calling these functions triggers the dynamic import without rendering
 * the component, so the JS chunk is already in the browser's cache when
 * the user actually navigates. Safe to call multiple times — the browser
 * deduplicates identical dynamic imports.
 */

let detailPrefetched = false;
let activityPrefetched = false;
let settingsPrefetched = false;
let addDialogPrefetched = false;

export function prefetchDetailChunk(): void {
  if (detailPrefetched) return;
  detailPrefetched = true;
  void import("../routes/ShowDetailPage");
}

export function prefetchActivityChunk(): void {
  if (activityPrefetched) return;
  activityPrefetched = true;
  void import("../routes/ActivityPage");
}

export function prefetchSettingsChunk(): void {
  if (settingsPrefetched) return;
  settingsPrefetched = true;
  void import("../routes/SettingsPage");
}

export function prefetchAddDialogChunk(): void {
  if (addDialogPrefetched) return;
  addDialogPrefetched = true;
  void import("../routes/AddAnimeDialog");
}
