import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";

const root = new URL("../", import.meta.url);

const violations = [];

function read(relPath) {
  return readFileSync(new URL(relPath, root), "utf8");
}

function assert(condition, message) {
  if (!condition) violations.push(message);
}

const appCss = read("./src/app.css");
const appCssLines = appCss.split(/\r?\n/).length;

assert(
  appCssLines <= 80,
  `Expected src/app.css to be minimal (<= 80 lines), found ${appCssLines}.`,
);

assert(
  !/:root\s*\{/.test(appCss),
  "Expected src/app.css to stop defining the bespoke :root design token system.",
);

const sourceFiles = [
  "src/App.svelte",
  "src/components/Navbar.svelte",
  "src/components/Toast.svelte",
  "src/pages/Shows.svelte",
  "src/pages/ShowDetail.svelte",
  "src/pages/Activity.svelte",
  "src/pages/Settings.svelte",
];

for (const relPath of sourceFiles) {
  const content = read(`./${relPath}`);

  assert(
    !/<style(\s|>)/.test(content),
    `Expected ${relPath} to have no page/component <style> block.`,
  );

  assert(
    !/node_modules\/@foxui\/core\/dist/.test(content),
    `Expected ${relPath} to import Fox UI from @foxui/core, not package internals.`,
  );

  assert(
    !/svelte-sonner/.test(content),
    `Expected ${relPath} to stop importing svelte-sonner directly.`,
  );
}

const primitiveImports = [
  "../components/Badge.svelte",
  "../components/Modal.svelte",
  "../components/Spinner.svelte",
  "../components/ui/Button.svelte",
  "../components/ui/Input.svelte",
  "../components/ui/Select.svelte",
  "../components/ui/Surface.svelte",
  "../components/ui/Toggle.svelte",
];

for (const relPath of [
  "src/App.svelte",
  "src/components/Navbar.svelte",
  "src/pages/Shows.svelte",
  "src/pages/ShowDetail.svelte",
  "src/pages/Activity.svelte",
  "src/pages/Settings.svelte",
]) {
  const content = read(`./${relPath}`);
  for (const importPath of primitiveImports) {
    assert(
      !content.includes(importPath),
      `Expected ${relPath} to stop importing retired local primitive ${importPath}.`,
    );
  }
}

const retiredComponentFiles = [
  "src/components/Badge.svelte",
  "src/components/Modal.svelte",
  "src/components/Spinner.svelte",
  "src/components/ui/Button.svelte",
  "src/components/ui/Input.svelte",
  "src/components/ui/Select.svelte",
  "src/components/ui/Surface.svelte",
  "src/components/ui/Toggle.svelte",
];

for (const relPath of retiredComponentFiles) {
  assert(
    !existsSync(join(root.pathname, relPath)),
    `Expected retired component file ${relPath} to be removed.`,
  );
}

if (violations.length > 0) {
  console.error("Fox-first migration checks failed:\n");
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

console.log("Fox-first migration checks passed.");
