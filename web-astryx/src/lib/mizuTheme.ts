import { defineTheme } from "@astryxdesign/core/theme";

/**
 * Mizu theme: ported from Skeleton's "Wintry" theme.
 *
 * Cool blue accent with slate/blue-gray surfaces. Uses the same OKLCH
 * color values as the original Skeleton theme for exact color matching.
 * Dark mode is the default look (matching the original app).
 */
export const mizuTheme = defineTheme({
  name: "mizu-wintry",

  color: {
    accent: "#1d4ed8",
  },

  tokens: {
    // ── Accent (Skeleton primary-500/400) ──────────────────────────
    "--color-accent": ["oklch(62.31% 0.19 259.81deg)", "oklch(68.61% 0.15 257.7deg)"],
    "--color-accent-muted": [
      "oklch(62.31% 0.19 259.81deg / 0.2)",
      "oklch(68.61% 0.15 257.7deg / 0.25)",
    ],
    "--color-text-accent": ["oklch(49.59% 0.18 261.77deg)", "oklch(75.45% 0.12 253.41deg)"],
    "--color-icon-accent": ["oklch(49.59% 0.18 261.77deg)", "oklch(68.61% 0.15 257.7deg)"],
    "--color-on-accent": ["#FFFFFF", "#FFFFFF"],

    // ── Surfaces (Skeleton surface scale, hue ~264 cool blue-gray) ─
    "--color-background-body": ["oklch(97.59% 0 264.7deg)", "oklch(18.53% 0.04 263.67deg)"],
    "--color-background-surface": ["oklch(88.12% 0.01 261.81deg)", "oklch(24.95% 0.04 264.15deg)"],
    "--color-background-card": ["#FFFFFF", "oklch(31.02% 0.04 264.35deg)"],
    "--color-background-popover": ["#FFFFFF", "oklch(31.02% 0.04 264.35deg)"],
    "--color-background-muted": ["oklch(78.51% 0.02 265.97deg)", "oklch(36.45% 0.05 264.44deg)"],
    "--color-background-inverted": ["oklch(18.53% 0.04 263.67deg)", "oklch(97.59% 0 264.7deg)"],

    // ── Text ────────────────────────────────────────────────────────
    "--color-text-primary": ["oklch(18.53% 0.04 263.67deg)", "oklch(97.59% 0 264.7deg)"],
    "--color-text-secondary": ["oklch(47.47% 0.05 264.53deg)", "oklch(68.49% 0.03 264.38deg)"],
    "--color-text-disabled": ["oklch(68.49% 0.03 264.38deg)", "oklch(42.05% 0.05 264.5deg)"],
    "--color-on-dark": "#FFFFFF",
    "--color-on-light": "oklch(18.53% 0.04 263.67deg)",

    // ── Icons ───────────────────────────────────────────────────────
    "--color-icon-primary": ["oklch(18.53% 0.04 263.67deg)", "oklch(97.59% 0 264.7deg)"],
    "--color-icon-secondary": ["oklch(47.47% 0.05 264.53deg)", "oklch(68.49% 0.03 264.38deg)"],
    "--color-icon-disabled": ["oklch(68.49% 0.03 264.38deg)", "oklch(42.05% 0.05 264.5deg)"],

    // ── Borders ─────────────────────────────────────────────────────
    "--color-border": ["oklch(78.51% 0.02 265.97deg)", "oklch(42.05% 0.05 264.5deg / 0.4)"],
    "--color-border-emphasized": ["oklch(58.25% 0.04 265.77deg)", "oklch(47.47% 0.05 264.53deg)"],

    // ── Overlays ────────────────────────────────────────────────────
    "--color-overlay": ["oklch(18.53% 0.04 263.67deg / 0.5)", "oklch(0% 0 0 / 0.6)"],
    "--color-overlay-hover": [
      "oklch(62.31% 0.19 259.81deg / 0.05)",
      "oklch(68.61% 0.15 257.7deg / 0.08)",
    ],
    "--color-overlay-pressed": [
      "oklch(62.31% 0.19 259.81deg / 0.1)",
      "oklch(68.61% 0.15 257.7deg / 0.12)",
    ],

    // ── Neutral overlay ─────────────────────────────────────────────
    "--color-neutral": ["oklch(18.53% 0.04 263.67deg / 0.06)", "oklch(97.59% 0 264.7deg / 0.1)"],

    // ── Status (Skeleton success/warning/error 500) ────────────────
    "--color-success": ["oklch(61.88% 0.18 135.65deg)", "oklch(69.44% 0.19 133.05deg)"],
    "--color-success-muted": [
      "oklch(61.88% 0.18 135.65deg / 0.2)",
      "oklch(69.44% 0.19 133.05deg / 0.25)",
    ],
    "--color-on-success": ["#FFFFFF", "oklch(18.53% 0.04 263.67deg)"],

    "--color-error": ["oklch(51.7% 0.2 358.74deg)", "oklch(57.22% 0.22 358.61deg)"],
    "--color-error-muted": [
      "oklch(51.7% 0.2 358.74deg / 0.2)",
      "oklch(57.22% 0.22 358.61deg / 0.25)",
    ],
    "--color-on-error": ["#FFFFFF", "#FFFFFF"],

    "--color-warning": ["oklch(70.68% 0.14 85.14deg)", "oklch(79.53% 0.16 86.04deg)"],
    "--color-warning-muted": [
      "oklch(70.68% 0.14 85.14deg / 0.2)",
      "oklch(79.53% 0.16 86.04deg / 0.25)",
    ],
    "--color-on-warning": ["oklch(18.53% 0.04 263.67deg)", "oklch(18.53% 0.04 263.67deg)"],

    // ── Effects ─────────────────────────────────────────────────────
    "--color-skeleton": ["oklch(78.51% 0.02 265.97deg)", "oklch(36.45% 0.05 264.44deg)"],
    "--color-shadow": ["oklch(18.53% 0.04 263.67deg / 0.1)", "oklch(0% 0 0 / 0.3)"],
    "--color-tint-hover": ["oklch(18.53% 0.04 263.67deg)", "oklch(97.59% 0 264.7deg)"],
  },
});
