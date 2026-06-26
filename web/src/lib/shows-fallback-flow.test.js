import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const showsSvelte = readFileSync(resolve(import.meta.dirname, "../pages/Shows.svelte"), "utf8");
const apiJs = readFileSync(resolve(import.meta.dirname, "./api.js"), "utf8");
const storesJs = readFileSync(resolve(import.meta.dirname, "./stores.js"), "utf8");
const settingsSvelte = readFileSync(
  resolve(import.meta.dirname, "../pages/Settings.svelte"),
  "utf8",
);

test("shows page exposes the AFL fallback branch in the add modal", () => {
  assert.match(showsSvelte, /Not in Sonarr\? Search AFL and add to Sonarr/);
  assert.match(showsSvelte, /lookupSonarrSeries/);
  assert.match(showsSvelte, /addSonarrSeries/);
  assert.match(showsSvelte, /manualMatchPending/);
  assert.match(showsSvelte, /searchAFL\(''\)/);
  assert.match(showsSvelte, /scheduleFallbackAflFilter/);
  assert.match(showsSvelte, /shouldAutoCreateMappingAfterSonarrAdd/);
  assert.match(showsSvelte, /await handleConfirmMatch\(\)/);
  assert.match(showsSvelte, /isCompactMatchSelection/);
  assert.match(showsSvelte, /Looks like an exact cached AFL match/);
  assert.match(showsSvelte, /Quality profile/);
  assert.match(showsSvelte, /Season folder/);
  assert.match(showsSvelte, /posterUrl\(result\)/);
  assert.match(showsSvelte, /disabled=\{sonarrAdding\}/);
  assert.doesNotMatch(showsSvelte, /Root folder:/);
  assert.doesNotMatch(showsSvelte, /Monitor mode:/);
  assert.doesNotMatch(showsSvelte, /result\.slug<\/div>/);
});

test("api exposes Sonarr lookup, add, and add-options endpoints", () => {
  assert.match(apiJs, /export function getSonarrAddOptions\(/);
  assert.match(apiJs, /export function lookupSonarrSeries\(/);
  assert.match(apiJs, /export function addSonarrSeries\(/);
});

test("settings page does not expose Sonarr add defaults controls", () => {
  assert.match(storesJs, /sonarr_add_root_folder/);
  assert.match(storesJs, /sonarr_add_quality_profile_id/);
  assert.match(storesJs, /sonarr_add_monitor_mode/);
  assert.match(storesJs, /sonarr_add_season_folder/);
  assert.doesNotMatch(settingsSvelte, /Sonarr add defaults/);
});
