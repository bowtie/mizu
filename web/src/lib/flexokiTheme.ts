import { defineTheme } from "@astryxdesign/core/theme";
import { neutralTheme } from "@astryxdesign/theme-neutral";

/**
 * Flexoki theme — ported from Steph Ango's Flexoki color system.
 * https://stephango.com/flexoki
 *
 * Warm monochromatic base (paper #FFFCF0 → black #100F0F) with 8 accent
 * colors. Light themes use the 600 accent stops; dark themes use 400.
 *
 * Extends neutralTheme for component styling, typography, motion, shadows.
 */
export const flexokiTheme = defineTheme({
  name: "flexoki",
  extends: neutralTheme,

  tokens: {
    // ── Backgrounds ─────────────────────────────────────────────────
    // Light: paper bg, base-100 ui, base-50 bg-2
    // Dark:  black bg, base-950 bg-2, base-900 ui, base-850 ui-2
    "--color-background-body": ["#F2F0E5", "#100F0F"],
    "--color-background-surface": ["#FFFCF0", "#1C1B1A"],
    "--color-background-card": ["#FFFCF0", "#1C1B1A"],
    "--color-background-popover": ["#FFFCF0", "#282726"],
    "--color-background-muted": ["#E6E4D9", "#282726"],

    // ── Accent (Flexoki blue) ───────────────────────────────────────
    // Light: blue-600 #205EA6 / Dark: blue-400 #4385BE
    "--color-accent": ["#205EA6", "#4385BE"],
    "--color-accent-muted": ["#205EA626", "#4385BE3F"],
    "--color-text-accent": ["#205EA6", "#4385BE"],
    "--color-icon-accent": ["#205EA6", "#4385BE"],
    "--color-on-accent": ["#FFFCF0", "#100F0F"],

    // ── Text ────────────────────────────────────────────────────────
    // Light: tx=black, tx-2=base-600, tx-3=base-300
    // Dark:  tx=base-200, tx-2=base-400 (base-500 was <4.5:1 on muted surfaces), tx-3=base-700
    "--color-text-primary": ["#100F0F", "#CECDC3"],
    "--color-text-secondary": ["#6F6E69", "#9F9D96"],
    "--color-text-disabled": ["#B7B5AC", "#575653"],
    "--color-on-dark": "#FFFCF0",
    "--color-on-light": "#100F0F",

    // ── Icons ───────────────────────────────────────────────────────
    "--color-icon-primary": ["#100F0F", "#CECDC3"],
    "--color-icon-secondary": ["#6F6E69", "#9F9D96"],
    "--color-icon-disabled": ["#B7B5AC", "#575653"],

    // ── Borders ─────────────────────────────────────────────────────
    // Light: ui-3=base-200, ui-2=base-150
    // Dark:  ui-3=base-800, ui-2=base-850
    "--color-border": ["#DAD8CE", "#403E3C"],
    "--color-border-emphasized": ["#CECDC3", "#575653"],

    // ── Overlays ────────────────────────────────────────────────────
    "--color-overlay": ["#100F0F80", "#100F0FCC"],
    "--color-overlay-hover": ["#100F0F0D", "#CECDC30D"],
    "--color-overlay-pressed": ["#100F0F1A", "#CECDC31A"],
    "--color-neutral": ["#100F0F0F", "#CECDC31A"],

    // ── Status (Flexoki accent 600 light / 400 dark) ────────────────
    // Green: #66800B / #879A39
    "--color-success": ["#66800B", "#879A39"],
    "--color-success-muted": ["#66800B26", "#879A393F"],
    "--color-on-success": ["#FFFCF0", "#100F0F"],

    // Red: #AF3029 / #D14D41
    "--color-error": ["#AF3029", "#D14D41"],
    "--color-error-muted": ["#AF302926", "#D14D413F"],
    "--color-on-error": ["#FFFCF0", "#100F0F"],

    // Yellow: #AD8301 / #D0A215
    "--color-warning": ["#AD8301", "#D0A215"],
    "--color-warning-muted": ["#AD830126", "#D0A2153F"],
    "--color-on-warning": ["#FFFCF0", "#100F0F"],

    // ── Effects ─────────────────────────────────────────────────────
    "--color-skeleton": ["#E6E4D9", "#343331"],
    "--color-shadow": ["#100F0F1A", "#00000066"],
    "--color-tint-hover": ["#100F0F", "#CECDC3"],

    // ── Categorical (Flexoki accents) ───────────────────────────────
    // Red #AF3029 / #D14D41
    "--color-background-red": ["#AF302926", "#D14D413F"],
    "--color-border-red": ["#AF3029", "#D14D41"],
    "--color-icon-red": ["#AF3029", "#D14D41"],
    "--color-text-red": ["#AF3029", "#D14D41"],

    // Orange #BC5215 / #DA702C
    "--color-background-orange": ["#BC521526", "#DA702C3F"],
    "--color-border-orange": ["#BC5215", "#DA702C"],
    "--color-icon-orange": ["#BC5215", "#DA702C"],
    "--color-text-orange": ["#BC5215", "#DA702C"],

    // Yellow #AD8301 / #D0A215
    "--color-background-yellow": ["#AD830126", "#D0A2153F"],
    "--color-border-yellow": ["#AD8301", "#D0A215"],
    "--color-icon-yellow": ["#AD8301", "#D0A215"],
    "--color-text-yellow": ["#AD8301", "#D0A215"],

    // Green #66800B / #879A39
    "--color-background-green": ["#66800B26", "#879A393F"],
    "--color-border-green": ["#66800B", "#879A39"],
    "--color-icon-green": ["#66800B", "#879A39"],
    "--color-text-green": ["#66800B", "#879A39"],

    // Cyan #24837B / #3AA99F
    "--color-background-teal": ["#24837B26", "#3AA99F3F"],
    "--color-border-teal": ["#24837B", "#3AA99F"],
    "--color-icon-teal": ["#24837B", "#3AA99F"],
    "--color-text-teal": ["#24837B", "#3AA99F"],

    // Cyan (same as teal in Flexoki)
    "--color-background-cyan": ["#24837B26", "#3AA99F3F"],
    "--color-border-cyan": ["#24837B", "#3AA99F"],
    "--color-icon-cyan": ["#24837B", "#3AA99F"],
    "--color-text-cyan": ["#24837B", "#3AA99F"],

    // Blue #205EA6 / #4385BE
    "--color-background-blue": ["#205EA626", "#4385BE3F"],
    "--color-border-blue": ["#205EA6", "#4385BE"],
    "--color-icon-blue": ["#205EA6", "#4385BE"],
    "--color-text-blue": ["#205EA6", "#92BFDB"],

    // Purple #5E409D / #8B7EC8
    "--color-background-purple": ["#5E409D26", "#8B7EC83F"],
    "--color-border-purple": ["#5E409D", "#8B7EC8"],
    "--color-icon-purple": ["#5E409D", "#8B7EC8"],
    "--color-text-purple": ["#5E409D", "#8B7EC8"],

    // Magenta/Pink #A02F6F / #CE5D97
    "--color-background-pink": ["#A02F6F26", "#CE5D973F"],
    "--color-border-pink": ["#A02F6F", "#CE5D97"],
    "--color-icon-pink": ["#A02F6F", "#CE5D97"],
    "--color-text-pink": ["#A02F6F", "#CE5D97"],

    // Gray (Flexoki base tones)
    "--color-background-gray": ["#E6E4D9", "#3433313F"],
    "--color-border-gray": ["#CECDC3", "#403E3C"],
    "--color-icon-gray": ["#6F6E69", "#878580"],
    "--color-text-gray": ["#100F0F", "#CECDC3"],
  },
});
