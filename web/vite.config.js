import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url';

const appEnvironmentShim = fileURLToPath(new URL('./src/shims/app-environment.js', import.meta.url));

export default defineConfig({
  resolve: {
    alias: {
      '$app/environment': appEnvironmentShim,
    },
  },
  optimizeDeps: {
    esbuildOptions: {
      plugins: [
        {
          name: 'foxui-sveltekit-shim',
          setup(build) {
            build.onResolve({ filter: /^\$app\/environment$/ }, () => ({
              path: appEnvironmentShim,
            }));
          },
        },
      ],
    },
  },
  plugins: [
    svelte(),
    tailwindcss(),
  ],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8978',
        changeOrigin: true,
      },
    },
  },
});
