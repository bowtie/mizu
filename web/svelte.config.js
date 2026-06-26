import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      fallback: 'index.html',
      pages: '../internal/web/dist',
      assets: '../internal/web/dist',
      precompress: false,
      strict: false,
    }),
    files: {
      routes: 'src/routes',
      appTemplate: 'src/app.html',
    },
  },
};

export default config;
