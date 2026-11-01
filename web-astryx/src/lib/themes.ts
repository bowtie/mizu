import type { DefinedTheme } from "@astryxdesign/core/theme";

import { neutralTheme } from "@astryxdesign/theme-neutral";
// neutralTheme CSS is loaded eagerly in main.tsx (default theme)

import { y2kTheme } from "@astryxdesign/theme-y2k";
// y2kTheme CSS is loaded eagerly in main.tsx (second default)

import { mizuWintryTheme } from "./mizu-wintry";
import { tokyoNightTheme } from "./tokyo-night";
import { flexokiTheme } from "./flexoki";
import { y2kRoundedTheme } from "./y2k-rounded";

// CSS for custom themes is loaded on-demand via loadThemeCSS() below,
// not imported eagerly — saves ~60KB from the initial CSS payload.

export interface ThemeOption {
  id: string;
  label: string;
  description: string;
  theme: DefinedTheme;
  mode?: "dark" | "light" | "system";
}

export const themeOptions: ThemeOption[] = [
  {
    id: "neutral",
    label: "Neutral",
    description: "Muted warm grays, system fonts",
    theme: neutralTheme,
  },
  {
    id: "y2k-rounded",
    label: "Y2K Rounded",
    description: "Periwinkle + pastels with soft rounded corners",
    theme: y2kRoundedTheme,
  },
  {
    id: "y2k",
    label: "Y2K",
    description: "Periwinkle + pastels, sharp brutalist edges",
    theme: y2kTheme,
  },
  {
    id: "mizu-wintry",
    label: "Mizu Wintry",
    description: "Skeleton Wintry ported — cool blue, dark mode",
    theme: mizuWintryTheme,
    mode: "dark",
  },
  {
    id: "tokyo-night",
    label: "Tokyo Night",
    description: "folke/tokyonight.nvim — deep navy bg, blue/purple accents",
    theme: tokyoNightTheme,
  },
  {
    id: "flexoki",
    label: "Flexoki",
    description: "Steph Ango — warm paper tones, inky text, muted accents",
    theme: flexokiTheme,
  },
];

export const defaultTheme = themeOptions[0];

// Track which theme CSS files have been loaded to avoid duplicate imports.
const loadedThemeCSS = new Set<string>(["neutral", "y2k"]);

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
  }
}
