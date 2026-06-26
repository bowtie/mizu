import { defineConfig, lazyPlugins } from "vite-plus";
import { sveltekit } from "@sveltejs/kit/vite";
import tailwindcss from "@tailwindcss/vite";

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
  plugins: lazyPlugins(() => [sveltekit(), tailwindcss()]),
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
