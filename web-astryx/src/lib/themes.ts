import type { DefinedTheme } from "@astryxdesign/core/theme";

import { neutralTheme } from "@astryxdesign/theme-neutral";
// neutralTheme CSS is loaded eagerly in main.tsx (default theme)

// y2k theme is lazy-loaded — only loaded if the user selects it.
// This saves ~18KB from the initial CSS payload and ~8KB from the
// JS bundle for users who stick with the neutral theme.

// CSS for custom themes is loaded on-demand via loadThemeCSS() below,
// not imported eagerly — saves ~60KB from the initial CSS payload.
// Theme JS definitions are also lazy-loaded via loadThemeDef() — only
// neutral and y2k are eager, the rest load when the user selects them.

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
    id: "neutral",
    label: "Neutral",
    description: "Muted warm grays, system fonts",
  },
  {
    id: "y2k-rounded",
    label: "Y2K Rounded",
    description: "Periwinkle + pastels with soft rounded corners",
  },
  {
    id: "y2k",
    label: "Y2K",
    description: "Periwinkle + pastels, sharp brutalist edges",
  },
  {
    id: "mizu-wintry",
    label: "Mizu Wintry",
    description: "Skeleton Wintry ported — cool blue, dark mode",
    mode: "dark",
  },
  {
    id: "tokyo-night",
    label: "Tokyo Night",
    description: "folke/tokyonight.nvim — deep navy bg, blue/purple accents",
  },
  {
    id: "flexoki",
    label: "Flexoki",
    description: "Steph Ango — warm paper tones, inky text, muted accents",
  },
];

// Eager theme map — only the default theme.
const eagerThemes: Record<string, DefinedTheme> = {
  neutral: neutralTheme,
};

// Cache for lazy-loaded theme definitions.
const themeDefCache = new Map<string, DefinedTheme>();

/** Load a theme definition (JS object with color tokens). Eager themes
 *  (neutral, y2k) are available immediately. Custom themes are dynamically
 *  imported on first use, keeping ~32KB of theme tokens out of the
 *  initial bundle. */
export async function loadThemeDef(themeId: string): Promise<DefinedTheme> {
  if (eagerThemes[themeId]) return eagerThemes[themeId];
  if (themeDefCache.has(themeId)) return themeDefCache.get(themeId)!;

  let theme: DefinedTheme;
  switch (themeId) {
    case "mizu-wintry": {
      const m = await import("./mizu-wintry");
      theme = m.mizuWintryTheme;
      break;
    }
    case "tokyo-night": {
      const m = await import("./tokyo-night");
      theme = m.tokyoNightTheme;
      break;
    }
    case "flexoki": {
      const m = await import("./flexoki");
      theme = m.flexokiTheme;
      break;
    }
    case "y2k-rounded": {
      const m = await import("./y2k-rounded");
      theme = m.y2kRoundedTheme;
      break;
    }
    case "y2k": {
      const m = await import("@astryxdesign/theme-y2k");
      theme = m.y2kTheme;
      break;
    }
    default:
      theme = neutralTheme;
  }
  themeDefCache.set(themeId, theme);
  return theme;
}

export const defaultTheme: ThemeOption = {
  id: "neutral",
  label: "Neutral",
  description: "Muted warm grays, system fonts",
  theme: neutralTheme,
};

// Track which theme CSS files have been loaded to avoid duplicate imports.
const loadedThemeCSS = new Set<string>(["neutral"]);

/** Dynamically load the CSS for a theme if it hasn't been loaded yet.
 *  The default themes (neutral, y2k) are loaded eagerly in main.tsx.
 *  Custom themes load their CSS on-demand when the user selects them. */
export async function loadThemeCSS(themeId: string): Promise<void> {
  if (loadedThemeCSS.has(themeId)) return;
  loadedThemeCSS.add(themeId);
  switch (themeId) {
    case "mizu-wintry":
      await import("./mizuTheme.css");
      break;
    case "tokyo-night":
      await import("./tokyoNightTheme.css");
      break;
    case "flexoki":
      await import("./flexokiTheme.css");
      break;
    case "y2k-rounded":
      await import("./y2kRoundedTheme.css");
      break;
    case "y2k":
      await import("@astryxdesign/theme-y2k/theme.css");
      break;
  }
}
