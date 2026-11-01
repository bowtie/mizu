import path from "path";
import { fileURLToPath } from "url";
import { defineConfig, lazyPlugins } from "vite-plus";
import react from "@vitejs/plugin-react";
import stylex from "@stylexjs/unplugin";

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
  fmt: {},
  lint: {},
  test: {
    include: ["src/**/*.test.ts"],
    // Run the whole suite under happy-dom so component tests can render
    // DOM without a per-file `@vitest-environment happy-dom` docblock.
    // The plain lib tests don't touch DOM APIs and pass unchanged under it.
    // Future node-only tests can opt back out with `@vitest-environment node`.
    environment: "happy-dom",
    // Shared per-file setup (sets React 19's IS_REACT_ACT_ENVIRONMENT flag).
    setupFiles: ["./src/test/setup.ts"],
  },
  plugins: lazyPlugins(() => [
    // Declare CSS layer order so theme overrides beat component base styles.
    {
      name: "astryx-css-layer-order",
      transformIndexHtml() {
        return [
          {
            tag: "style",
            children:
              "@layer reset, priority1, priority2, priority3, priority4, priority5, priority6, priority7, priority8, priority9, astryx-theme;",
            injectTo: "head-prepend",
          },
        ];
      },
    },
    // Preload the ShowsPage chunk in the built HTML so it starts
    // downloading in parallel with the main bundle. Without this,
    // ShowsPage only loads after the main bundle parses and fires
    // `void import("./routes/ShowsPage")` — a sequential waterfall.
    // The landing page is always the first thing users see, so
    // parallelizing its download saves ~50ms on first visit.
    //
    // Also conditionally preloads the ShowDetailPage chunk when the
    // user is on a /shows/:id URL — the chunk is 33KB and would
    // otherwise only start downloading after the main bundle parses.
    {
      name: "preload-route-chunks",
      apply: "build",
      transformIndexHtml: {
        order: "post" as const,
        handler(
          html: string,
          ctx: { bundle?: Record<string, { name?: string; fileName: string }> },
        ) {
          if (!ctx.bundle) return html;
          let showsPageUrl = "";
          let detailPageUrl = "";
          let cssUrl = "";
          for (const chunk of Object.values(ctx.bundle)) {
            if (chunk.name === "ShowsPage") showsPageUrl = "/" + chunk.fileName;
            if (chunk.name === "ShowDetailPage") detailPageUrl = "/" + chunk.fileName;
            // The main CSS chunk doesn't have a name — detect by .css extension.
            if (!cssUrl && chunk.fileName.endsWith(".css")) cssUrl = "/" + chunk.fileName;
          }

          // Move the CSS stylesheet link to before the inline API prefetch
          // script so the browser starts downloading the 168KB stylesheet
          // immediately, in parallel with the JS bundle. Vite places the
          // stylesheet after the script tag by default, which delays CSS
          // download by the time it takes to parse the script tag.
          if (cssUrl) {
            const stylesheetLink = `<link rel="stylesheet" crossorigin href="${cssUrl}">`;
            // Remove the stylesheet from its current position (after script tag)
            html = html.replace(stylesheetLink, "");
            // Insert it before the first <script> tag in the head
            html = html.replace("<script>", `${stylesheetLink}\n    <script>`);
          }

          let extra = "";
          if (showsPageUrl) {
            extra += `  <link rel="modulepreload" crossorigin href="${showsPageUrl}">\n`;
          }
          if (detailPageUrl) {
            // Inject a tiny inline script that creates a modulepreload
            // link for ShowDetailPage only when on /shows/:id. This
            // starts the 33KB chunk download in parallel with the main
            // bundle on hard refresh of a detail page.
            extra += `  <script>if(location.pathname.match(/^\\/shows\\/\\d+/)){var l=document.createElement("link");l.rel="modulepreload";l.crossOrigin="anonymous";l.href="${detailPageUrl}";document.head.appendChild(l);}</script>\n`;
          }
          if (extra) {
            return html.replace("</head>", extra + "  </head>");
          }
          return html;
        },
      },
    },
    stylex.vite({
      dev: process.env.NODE_ENV === "development",
      runtimeInjection: false,
      treeshakeCompensation: true,
      useCSSLayers: true,
      unstable_moduleResolution: {
        type: "commonJS",
        rootDir: __dirname,
      },
      lightningcssOptions: {
        targets: lightningcssTargets,
      },
    }),
    react(),
  ]),
  resolve: {
    alias: {
      "@astryxdesign/core/theme/tokens.stylex": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/core/src/theme/tokens.stylex.ts",
      ),
      "@astryxdesign/core/astryx.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/core/dist/astryx.css",
      ),
      "@astryxdesign/core/reset.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/core/src/reset.css",
      ),
      "@astryxdesign/theme-neutral/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-neutral/dist/theme.css",
      ),
      "@astryxdesign/theme-y2k/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-y2k/dist/theme.css",
      ),
      "@astryxdesign/theme-butter/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-butter/dist/theme.css",
      ),
      "@astryxdesign/theme-chocolate/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-chocolate/dist/theme.css",
      ),
      "@astryxdesign/theme-gothic/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-gothic/dist/theme.css",
      ),
      "@astryxdesign/theme-matcha/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-matcha/dist/theme.css",
      ),
      "@astryxdesign/theme-stone/theme.css": path.resolve(
        __dirname,
        "node_modules/@astryxdesign/theme-stone/dist/theme.css",
      ),
      "@astryxdesign/core": path.resolve(__dirname, "node_modules/@astryxdesign/core/src"),
      "@": path.resolve(__dirname, "src"),
    },
  },
  // Prevent Vite from pre-bundling Astryx with esbuild. Astryx ships as source
  // that must be compiled by the StyleX plugin — pre-bundling strips the
  // stylex.create/defineVars calls and causes a runtime error.
  optimizeDeps: {
    exclude: ["@astryxdesign/core", "@astryxdesign/theme-neutral"],
  },
  server: {
    port: 3001,
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://localhost:8978",
        changeOrigin: true,
      },
    },
  },
});
