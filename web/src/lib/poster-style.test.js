import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const appCss = readFileSync(resolve(import.meta.dirname, '../app.css'), 'utf8');
const showsSvelte = readFileSync(resolve(import.meta.dirname, '../pages/Shows.svelte'), 'utf8');

test('poster image keeps Fox-style utility transition classes', () => {
  assert.match(showsSvelte, /poster-image[\s\S]*opacity-0 transition-all duration-500 ease-in-out group-hover:scale-105/);
});

test('poster image css does not override transition properties', () => {
  const block = appCss.match(/\.poster-image\s*\{([\s\S]*?)\}/)?.[1] ?? '';
  assert.ok(block.length > 0, 'expected .poster-image block in app.css');
  assert.doesNotMatch(block, /transition/i);
});
