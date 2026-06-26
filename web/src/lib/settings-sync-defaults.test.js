import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const settingsSvelte = readFileSync(
  resolve(import.meta.dirname, "../pages/Settings.svelte"),
  "utf8",
);

test("settings page groups config into Connection, Sync, and Defaults sections", () => {
  assert.match(settingsSvelte, />Connection</);
  assert.match(settingsSvelte, />Sync</);
  assert.match(settingsSvelte, />Defaults</);
  assert.doesNotMatch(settingsSvelte, />Sync defaults</);
  assert.doesNotMatch(settingsSvelte, />Cadence</);
  assert.doesNotMatch(settingsSvelte, />Monthly refresh</);
});
