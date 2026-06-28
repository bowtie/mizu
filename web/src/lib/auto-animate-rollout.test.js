import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const packageJson = JSON.parse(
  readFileSync(resolve(import.meta.dirname, "../../package.json"), "utf8"),
);
const showsSvelte = readFileSync(resolve(import.meta.dirname, "../routes/shows/+page.svelte"), "utf8");
const activitySvelte = readFileSync(
  resolve(import.meta.dirname, "../routes/activity/+page.svelte"),
  "utf8",
);

test("frontend dependencies include AutoAnimate", () => {
  assert.equal(packageJson.dependencies["@formkit/auto-animate"], "^0.9.0");
});

test("shows page wires AutoAnimate into the poster grid", () => {
  assert.match(
    showsSvelte,
    /import\s+\{\s*autoAnimateAction as autoAnimate\s*\}\s+from\s+'\.\.\/\.\.\/lib\/autoAnimate\.js'/,
  );
  assert.match(showsSvelte, /use:autoAnimate/);
});

test("activity page uses slide transitions for expanded detail region", () => {
  assert.match(activitySvelte, /import\s+\{\s*slide\s*\}\s+from\s+'svelte\/transition'/);
  assert.match(activitySvelte, /transition:slide=\{\{\s*duration:\s*220\s*\}\}/);
  assert.doesNotMatch(
    activitySvelte,
    /Open details to inspect which anime changed during this run\./,
  );
  assert.doesNotMatch(activitySvelte, /border-t border-base-800\/80 pt-4/);
  assert.match(activitySvelte, /use:autoAnimate/);
});
