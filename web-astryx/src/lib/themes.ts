import { defineTheme } from '@astryxdesign/core/theme';
import type { DefinedTheme } from '@astryxdesign/core/theme';

import { neutralTheme } from '@astryxdesign/theme-neutral';
import { y2kTheme } from '@astryxdesign/theme-y2k';

import { mizuTheme } from './mizuTheme';
import { tokyoNightTheme } from './tokyoNightTheme';
import { flexokiTheme } from './flexokiTheme';

/** Y2K colors but with rounded corners instead of sharp/brutalist. */
const y2kRoundedTheme = defineTheme({
  name: 'y2k-rounded',
  extends: y2kTheme,
  radius: { base: 6, multiplier: 1 },
  tokens: {
    '--radius-none': '0px',
    '--radius-inner': '4px',
    '--radius-element': '8px',
    '--radius-container': '12px',
    '--radius-page': '16px',
    '--radius-full': '9999px',
  },
  components: {
    button: {
      base: {
        borderRadius: '8px',
        borderWidth: '1px',
        borderStyle: 'solid',
        borderColor: 'var(--color-border)',
      },
    },
    badge: {
      base: {
        borderRadius: '9999px',
      },
    },
    card: {
      base: {
        borderRadius: '12px',
      },
    },
    section: {
      base: {
        borderRadius: '12px',
      },
    },
  },
});

export interface ThemeOption {
  id: string;
  label: string;
  description: string;
  theme: DefinedTheme;
  mode?: 'dark' | 'light' | 'system';
}

export const themeOptions: ThemeOption[] = [
  {
    id: 'neutral',
    label: 'Neutral',
    description: 'Muted warm grays, system fonts',
    theme: neutralTheme,
  },
  {
    id: 'y2k-rounded',
    label: 'Y2K Rounded',
    description: 'Periwinkle + pastels with soft rounded corners',
    theme: y2kRoundedTheme,
  },
  {
    id: 'y2k',
    label: 'Y2K',
    description: 'Periwinkle + pastels, sharp brutalist edges',
    theme: y2kTheme,
  },
  {
    id: 'mizu-wintry',
    label: 'Mizu Wintry',
    description: 'Skeleton Wintry ported — cool blue, dark mode',
    theme: mizuTheme,
    mode: 'dark',
  },
  {
    id: 'tokyo-night',
    label: 'Tokyo Night',
    description: 'folke/tokyonight.nvim — deep navy bg, blue/purple accents',
    theme: tokyoNightTheme,
  },
  {
    id: 'flexoki',
    label: 'Flexoki',
    description: 'Steph Ango — warm paper tones, inky text, muted accents',
    theme: flexokiTheme,
  },
];

export const defaultTheme = themeOptions[0];
