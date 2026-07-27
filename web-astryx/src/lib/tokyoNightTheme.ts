import { defineTheme } from '@astryxdesign/core/theme';
import { neutralTheme } from '@astryxdesign/theme-neutral';

/**
 * Tokyo Night theme — ported from folke/tokyonight.nvim.
 *
 * Extends the Neutral theme (component styling, typography, motion, shadows)
 * and overrides only the color tokens with the Tokyo Night palette.
 *
 * Dark mode is the canonical look (the original Tokyo Night is dark-first).
 * Light mode uses the Tokyo Night Light palette.
 *
 * Source: https://github.com/folke/tokyonight.nvim
 */
export const tokyoNightTheme = defineTheme({
  name: 'tokyo-night',
  extends: neutralTheme,

  tokens: {
    // ── Backgrounds ─────────────────────────────────────────────────
    // Dark:  body #1a1b26, surface #16161e (lifted panels), card/popover #16161e
    // Light: body #d5d6db, surface #cbccd1, card/popover #cbccd1
    '--color-background-body': ['#d5d6db', '#1a1b26'],
    '--color-background-surface': ['#cbccd1', '#16161e'],
    '--color-background-card': ['#cbccd1', '#16161e'],
    '--color-background-popover': ['#cbccd1', '#16161e'],
    '--color-background-muted': ['#d3d4db', '#292e42'],

    // ── Accent (Tokyo Night blue) ───────────────────────────────────
    // Dark:  #7aa2f7 (the signature Tokyo Night blue)
    // Light: #2e7de9
    '--color-accent': ['#2e7de9', '#7aa2f7'],
    '--color-accent-muted': ['#2e7de933', '#7aa2f73F'],
    '--color-text-accent': ['#2e7de9', '#7aa2f7'],
    '--color-icon-accent': ['#2e7de9', '#7aa2f7'],
    '--color-on-accent': ['#ffffff', '#1a1b26'],

    // ── Text ────────────────────────────────────────────────────────
    // Dark:  fg #c0caf5, fg_dark/gutter #a9b1d6, comment #565f89
    // Light: fg #343b58, fg_dark #4c566a, comment #8489bb
    '--color-text-primary': ['#343b58', '#c0caf5'],
    '--color-text-secondary': ['#4c566a', '#a9b1d6'],
    '--color-text-disabled': ['#9698a3', '#565f89'],
    '--color-on-dark': '#ffffff',
    '--color-on-light': '#343b58',

    // ── Icons ───────────────────────────────────────────────────────
    '--color-icon-primary': ['#343b58', '#c0caf5'],
    '--color-icon-secondary': ['#4c566a', '#a9b1d6'],
    '--color-icon-disabled': ['#9698a3', '#565f89'],

    // ── Borders ─────────────────────────────────────────────────────
    // Dark:  fg_gutter #3b4261, dark3 #414868
    // Light: fg_gutter #a8aeb9, dark3 #9698a3
    '--color-border': ['#a8aeb9', '#3b4261'],
    '--color-border-emphasized': ['#9698a3', '#414868'],

    // ── Overlays ────────────────────────────────────────────────────
    '--color-overlay': ['#343b5880', '#000000CC'],
    '--color-overlay-hover': ['#343b580D', '#7aa2f71A'],
    '--color-overlay-pressed': ['#343b581A', '#7aa2f733'],
    '--color-neutral': ['#343b580F', '#c0caf51A'],

    // ── Status ──────────────────────────────────────────────────────
    // Dark:  green #9ece6a, red #f7768e, yellow #e0af68, orange #ff9e64
    // Light: green #3fa374, red #f7768e, yellow #e0af68, orange #e6914a
    '--color-success': ['#3fa374', '#9ece6a'],
    '--color-success-muted': ['#3fa37433', '#9ece6a3D'],
    '--color-on-success': ['#ffffff', '#1a1b26'],

    '--color-error': ['#db4b4b', '#f7768e'],
    '--color-error-muted': ['#db4b4b33', '#f7768e3D'],
    '--color-on-error': ['#ffffff', '#1a1b26'],

    '--color-warning': ['#e6914a', '#e0af68'],
    '--color-warning-muted': ['#e6914a33', '#e0af683D'],
    '--color-on-warning': ['#ffffff', '#1a1b26'],

    // ── Effects ─────────────────────────────────────────────────────
    '--color-skeleton': ['#d3d4db', '#292e42'],
    '--color-shadow': ['#343b581A', '#00000066'],
    '--color-tint-hover': ['#343b58', '#c0caf5'],

    // ── Categorical (Tokyo Night palette) ───────────────────────────
    // Red #f7768e / #db4b4b
    '--color-background-red': ['#f7768e33', '#f7768e3D'],
    '--color-border-red': ['#db4b4b', '#f7768e'],
    '--color-icon-red': ['#db4b4b', '#f7768e'],
    '--color-text-red': ['#db4b4b', '#f7768e'],

    // Orange #ff9e64 / #e6914a
    '--color-background-orange': ['#e6914a33', '#ff9e643D'],
    '--color-border-orange': ['#e6914a', '#ff9e64'],
    '--color-icon-orange': ['#e6914a', '#ff9e64'],
    '--color-text-orange': ['#e6914a', '#ff9e64'],

    // Yellow #e0af68
    '--color-background-yellow': ['#e0af6833', '#e0af683D'],
    '--color-border-yellow': ['#e0af68', '#e0af68'],
    '--color-icon-yellow': ['#e0af68', '#e0af68'],
    '--color-text-yellow': ['#e0af68', '#e0af68'],

    // Green #9ece6a / #3fa374
    '--color-background-green': ['#3fa37433', '#9ece6a3D'],
    '--color-border-green': ['#3fa374', '#9ece6a'],
    '--color-icon-green': ['#3fa374', '#9ece6a'],
    '--color-text-green': ['#3fa374', '#9ece6a'],

    // Teal #1abc9c
    '--color-background-teal': ['#1abc9c33', '#1abc9c3D'],
    '--color-border-teal': ['#1abc9c', '#1abc9c'],
    '--color-icon-teal': ['#1abc9c', '#1abc9c'],
    '--color-text-teal': ['#1abc9c', '#1abc9c'],

    // Cyan #7dcfff / #3ba4e9
    '--color-background-cyan': ['#3ba4e933', '#7dcfff3D'],
    '--color-border-cyan': ['#3ba4e9', '#7dcfff'],
    '--color-icon-cyan': ['#3ba4e9', '#7dcfff'],
    '--color-text-cyan': ['#3ba4e9', '#7dcfff'],

    // Blue #7aa2f7 / #2e7de9
    '--color-background-blue': ['#2e7de933', '#7aa2f73D'],
    '--color-border-blue': ['#2e7de9', '#7aa2f7'],
    '--color-icon-blue': ['#2e7de9', '#7aa2f7'],
    '--color-text-blue': ['#2e7de9', '#7aa2f7'],

    // Purple #9d7cd8 / #8c4bf0
    '--color-background-purple': ['#8c4bf033', '#9d7cd83D'],
    '--color-border-purple': ['#8c4bf0', '#9d7cd8'],
    '--color-icon-purple': ['#8c4bf0', '#9d7cd8'],
    '--color-text-purple': ['#8c4bf0', '#9d7cd8'],

    // Magenta/Pink #bb9af7 / #9854f1
    '--color-background-pink': ['#9854f133', '#bb9af73D'],
    '--color-border-pink': ['#9854f1', '#bb9af7'],
    '--color-icon-pink': ['#9854f1', '#bb9af7'],
    '--color-text-pink': ['#9854f1', '#bb9af7'],

    // Gray (dark3 #414868 / #9698a3)
    '--color-background-gray': ['#9698a333', '#4148683D'],
    '--color-border-gray': ['#9698a3', '#414868'],
    '--color-icon-gray': ['#9698a3', '#414868'],
    '--color-text-gray': ['#9698a3', '#414868'],
  },
});
