import { defineConfig, lazyPlugins } from "vite-plus";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

const appEnvironmentShim = fileURLToPath(
  new URL("./src/shims/app-environment.js", import.meta.url),
);

export default defineConfig({
  fmt: {},
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
  },
  test: {
    include: ["src/**/*.test.js"],
  },
  resolve: {
    alias: {
      "$app/environment": appEnvironmentShim,
    },
  },
  plugins: lazyPlugins(() => [svelte(), tailwindcss()]),
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8978",
        changeOrigin: true,
      },
    },
  },
});
