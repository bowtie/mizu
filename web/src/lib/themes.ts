import type { DefinedTheme } from "@astryxdesign/core/theme";

import { flexokiTheme } from "./flexoki";
// flexoki CSS is loaded eagerly in main.tsx (default theme)

// CSS for custom themes is loaded on-demand via loadThemeCSS() below,
// not imported eagerly. Theme JS definitions are also lazy-loaded via
// loadThemeDef() — only flexoki is eager, the rest load when the user
// selects them.

export interface ThemeOption {
  id: string;
  label: string;
  description: string;
  theme: DefinedTheme;
  mode?: "dark" | "light" | "system";
}

/** Lightweight metadata for the theme menu — no theme JS loaded. */
export interface ThemeMeta {
  id: string;
  label: string;
  description: string;
  mode?: "dark" | "light" | "system";
}

export const themeMetas: ThemeMeta[] = [
  {
    id: "flexoki",
    label: "Flexoki",
    description: "Warm paper tones and inky text, from Steph Ango",
  },
  {
    id: "tokyo-night",
    label: "Tokyo Night",
    description: "Deep navy with blue and purple accents, from folke/tokyonight.nvim",
  },
];

// Eager theme map — only the default theme.
const eagerThemes: Record<string, DefinedTheme> = {
  flexoki: flexokiTheme,
};

// Cache for lazy-loaded theme definitions.
const themeDefCache = new Map<string, DefinedTheme>();

/** Load a theme definition (JS object with color tokens). The eager
 *  theme (flexoki) is available immediately. Custom themes are
 *  dynamically imported on first use. */
export async function loadThemeDef(themeId: string): Promise<DefinedTheme> {
  if (eagerThemes[themeId]) return eagerThemes[themeId];
  if (themeDefCache.has(themeId)) return themeDefCache.get(themeId)!;

  let theme: DefinedTheme;
  switch (themeId) {
    case "tokyo-night": {
      const m = await import("./tokyo-night");
      theme = m.tokyoNightTheme;
      break;
    }
    default:
      theme = flexokiTheme;
  }
  themeDefCache.set(themeId, theme);
  return theme;
}

export const defaultTheme: ThemeOption = {
  id: "flexoki",
  label: "Flexoki",
  description: "Warm paper tones and inky text, from Steph Ango",
  theme: flexokiTheme,
};

// Track which theme CSS files have been loaded to avoid duplicate imports.
const loadedThemeCSS = new Set<string>(["flexoki"]);

/** Dynamically load the CSS for a theme if it hasn't been loaded yet.
 *  The default theme (flexoki) is loaded eagerly in main.tsx.
 *  Custom themes load their CSS on-demand when the user selects them. */
export async function loadThemeCSS(themeId: string): Promise<void> {
  if (loadedThemeCSS.has(themeId)) return;
  switch (themeId) {
    case "tokyo-night":
      await import("./tokyoNightTheme.css");
      break;
  }
  // Only mark as loaded after the import succeeds — a failed import
  // (network error, missing file) should be retryable on next call.
  loadedThemeCSS.add(themeId);
}
