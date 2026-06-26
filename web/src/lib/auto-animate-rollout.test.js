import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const packageJson = JSON.parse(
  readFileSync(resolve(import.meta.dirname, "../../package.json"), "utf8"),
);
const showsSvelte = readFileSync(resolve(import.meta.dirname, "../pages/Shows.svelte"), "utf8");
const activitySvelte = readFileSync(
  resolve(import.meta.dirname, "../pages/Activity.svelte"),
  "utf8",
);

test("frontend dependencies include AutoAnimate", () => {
  assert.equal(packageJson.dependencies["@formkit/auto-animate"], "^0.9.0");
});

test("shows page does not wire AutoAnimate into the poster grid or modal body", () => {
  assert.doesNotMatch(
    showsSvelte,
    /import\s+\{\s*autoAnimate\s*\}\s+from\s+'@formkit\/auto-animate'/,
  );
  assert.doesNotMatch(showsSvelte, /use:autoAnimate/);
});

test("activity page wires AutoAnimate into the log list and expanded detail region", () => {
  assert.match(activitySvelte, /import\s+\{\s*autoAnimate\s*\}\s+from\s+'@formkit\/auto-animate'/);
  assert.match(activitySvelte, /import\s+\{\s*slide\s*\}\s+from\s+'svelte\/transition'/);
  assert.match(activitySvelte, /<section class="space-y-4"\s+use:autoAnimate>/);
  assert.match(activitySvelte, /transition:slide=\{\{\s*duration:\s*220\s*\}\}/);
  assert.doesNotMatch(
    activitySvelte,
    /Open details to inspect which anime changed during this run\./,
  );
  assert.doesNotMatch(activitySvelte, /border-t border-base-800\/80 pt-4/);
});
