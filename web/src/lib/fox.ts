import type { BadgeVariant } from './components/ui/badge/index.ts';

export interface BadgeMeta {
  label: string;
  variant: BadgeVariant;
}

export const episodeTypeMeta: Record<string, BadgeMeta> = {
  canon: { label: 'Manga', variant: 'default' },
  mixed_canon: { label: 'Mixed', variant: 'warning' },
  mixed: { label: 'Mixed', variant: 'warning' },
  anime_canon: { label: 'Anime', variant: 'info' },
  filler: { label: 'Filler', variant: 'destructive' },
  special: { label: 'Special', variant: 'secondary' },
  unknown: { label: 'Unknown', variant: 'secondary' },
};

export function episodeTypeBadge(type: string | undefined | null): BadgeMeta {
  return episodeTypeMeta[type || 'unknown'] || episodeTypeMeta.unknown;
}

export function runStatusBadge(status: string | undefined | null): BadgeMeta {
  switch (status) {
    case 'success':
      return { label: 'Success', variant: 'success' };
    case 'partial':
      return { label: 'Partial', variant: 'warning' };
    case 'failed':
      return { label: 'Failed', variant: 'destructive' };
    case 'running':
      return { label: 'Running', variant: 'info' };
    default:
      return { label: status || 'Unknown', variant: 'secondary' };
  }
}

export function itemStatusBadge(status: string | undefined | null): BadgeMeta {
  switch (status) {
    case 'success':
      return { label: 'Success', variant: 'success' };
    case 'failed':
      return { label: 'Failed', variant: 'destructive' };
    default:
      return { label: status || 'Info', variant: 'secondary' };
  }
}

export function syncStateBadge(enabled: boolean): BadgeMeta {
  return enabled
    ? { label: 'Syncing', variant: 'success' }
    : { label: 'Paused', variant: 'secondary' };
}

export function monitorStateBadge(
  monitored: boolean,
  yesLabel: string = 'Monitored',
  noLabel: string = 'Unmonitored',
): BadgeMeta {
  return monitored
    ? { label: yesLabel, variant: 'success' }
    : { label: noLabel, variant: 'secondary' };
}
