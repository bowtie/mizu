import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const appCss = readFileSync(resolve(import.meta.dirname, "../app.css"), "utf8");

test("tailwindcss-animate is enabled in the app stylesheet", () => {
  assert.match(appCss, /@plugin "tailwindcss-animate";/);
});

test("tw-animate-css is imported for dialog/popover animation utilities", () => {
  assert.match(appCss, /@import "tw-animate-css"/);
});
