import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const activitySvelte = readFileSync(
  resolve(import.meta.dirname, "../pages/Activity.svelte"),
  "utf8",
);

test("activity page requests eight log entries per page", () => {
  assert.match(activitySvelte, /const PAGE_SIZE = 8;/);
});
