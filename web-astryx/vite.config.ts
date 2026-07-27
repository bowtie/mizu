import path from 'path';
import { fileURLToPath } from 'url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import stylex from '@stylexjs/unplugin';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Required: tells the StyleX plugin's internal lightningcss transform
// not to lower light-dark() into broken polyfill variables.
// Astryx tokens use light-dark() which is baseline 2024.
const lightningcssTargets = {
  chrome: 123 << 16,
  firefox: 120 << 16,
  safari: (17 << 16) | (5 << 8),
};

export default defineConfig({
  plugins: [
    // Declare CSS layer order so theme overrides beat component base styles.
    {
      name: 'astryx-css-layer-order',
      transformIndexHtml() {
        return [
          {
            tag: 'style',
            children:
              '@layer reset, priority1, priority2, priority3, priority4, priority5, priority6, priority7, priority8, priority9, astryx-theme;',
            injectTo: 'head-prepend',
          },
        ];
      },
    },
    stylex.vite({
      dev: process.env.NODE_ENV === 'development',
      runtimeInjection: false,
      treeshakeCompensation: true,
      useCSSLayers: true,
      unstable_moduleResolution: {
        type: 'commonJS',
        rootDir: __dirname,
      },
      lightningcssOptions: {
        targets: lightningcssTargets,
      },
    }),
    react(),
  ],
  resolve: {
    alias: {
      '@astryxdesign/core/theme/tokens.stylex': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/core/src/theme/tokens.stylex.ts',
      ),
      '@astryxdesign/core/astryx.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/core/dist/astryx.css',
      ),
      '@astryxdesign/core/reset.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/core/src/reset.css',
      ),
      '@astryxdesign/theme-neutral/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-neutral/dist/theme.css',
      ),
      '@astryxdesign/theme-y2k/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-y2k/dist/theme.css',
      ),
      '@astryxdesign/theme-butter/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-butter/dist/theme.css',
      ),
      '@astryxdesign/theme-chocolate/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-chocolate/dist/theme.css',
      ),
      '@astryxdesign/theme-gothic/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-gothic/dist/theme.css',
      ),
      '@astryxdesign/theme-matcha/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-matcha/dist/theme.css',
      ),
      '@astryxdesign/theme-stone/theme.css': path.resolve(
        __dirname,
        'node_modules/@astryxdesign/theme-stone/dist/theme.css',
      ),
      '@astryxdesign/core': path.resolve(__dirname, 'node_modules/@astryxdesign/core/src'),
      '@': path.resolve(__dirname, 'src'),
    },
  },
  // Prevent Vite from pre-bundling Astryx with esbuild. Astryx ships as source
  // that must be compiled by the StyleX plugin — pre-bundling strips the
  // stylex.create/defineVars calls and causes a runtime error.
  optimizeDeps: {
    exclude: ['@astryxdesign/core', '@astryxdesign/theme-neutral'],
  },
  server: {
    port: 3001,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8978',
        changeOrigin: true,
      },
    },
  },
});
