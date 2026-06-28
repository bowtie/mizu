import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const showsSvelte = readFileSync(resolve(import.meta.dirname, "../routes/shows/+page.svelte"), "utf8");

test("shows page uses a local centered add modal overlay instead of a shared modal component", () => {
  assert.match(showsSvelte, /import\s+\{\s*fade,\s*scale,\s*fly\s*\}\s+from\s+'svelte\/transition'/);
  assert.match(showsSvelte, /show-add-modal-shell/);
  assert.match(showsSvelte, /show-add-modal-panel/);
  assert.match(showsSvelte, /show-add-modal-close/);
  assert.doesNotMatch(showsSvelte, /<Modal\b/);
});

test("shows add modal keeps a restrained surface instead of the custom gradient treatment", () => {
  assert.doesNotMatch(showsSvelte, /bg-\[radial-gradient/);
  assert.doesNotMatch(showsSvelte, /Sonarr import/);
  assert.match(showsSvelte, /show-add-modal-panel[\s\S]*bg-popover/);
});

test("shows add modal renders the Sonarr chooser as a searchable poster list", () => {
  assert.match(showsSvelte, /let seriesQuery = ''/);
  assert.match(showsSvelte, /placeholder="Search Sonarr anime"/);
  assert.match(showsSvelte, /getSonarrSeries\(true\)/);
  assert.match(showsSvelte, /poster-thumb relative aspect-\[2\/3\]/);
  assert.match(showsSvelte, /posterUrl\(series\)/);
  assert.match(showsSvelte, /createPosterMonogram\(series\.title\)/);
});
