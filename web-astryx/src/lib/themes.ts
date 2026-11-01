import type { DefinedTheme } from "@astryxdesign/core/theme";

import { neutralTheme } from "@astryxdesign/theme-neutral";
import { y2kTheme } from "@astryxdesign/theme-y2k";

import { mizuWintryTheme } from "./mizu-wintry";
import "./mizuTheme.css";
import { tokyoNightTheme } from "./tokyo-night";
import "./tokyoNightTheme.css";
import { flexokiTheme } from "./flexoki";
import "./flexokiTheme.css";
import { y2kRoundedTheme } from "./y2k-rounded";
import "./y2kRoundedTheme.css";

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
