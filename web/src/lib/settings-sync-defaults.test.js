import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const settingsSvelte = readFileSync(
  resolve(import.meta.dirname, "../pages/Settings.svelte"),
  "utf8",
);

test("settings page keeps sync defaults flat and avoids redundant sync stats", () => {
  assert.match(settingsSvelte, />Sync defaults</);
  assert.doesNotMatch(settingsSvelte, />Cadence</);
  assert.doesNotMatch(settingsSvelte, />Monthly refresh</);
  assert.match(
    settingsSvelte,
    /<h3 class="text-base font-semibold text-base-50">Default tracking rules<\/h3>/,
  );
});
