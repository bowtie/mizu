import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const appCss = readFileSync(resolve(import.meta.dirname, "../app.css"), "utf8");

test("tailwindcss-animate is enabled in the app stylesheet", () => {
  assert.match(appCss, /@plugin "tailwindcss-animate";/);
});

test("fox modal has a fallback animation path based on dialog state attributes", () => {
  assert.match(appCss, /\[data-dialog-overlay]\[data-state="open"]/);
  assert.match(appCss, /\[data-dialog-overlay]\[data-state="closed"]/);
  assert.match(appCss, /\[data-dialog-content]\[data-state="open"]/);
  assert.match(appCss, /\[data-dialog-content]\[data-state="closed"]/);
});
