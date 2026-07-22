export interface BadgeMeta {
  label: string;
}

export const episodeTypeMeta: Record<string, BadgeMeta> = {
  canon: { label: 'Manga' },
  mixed_canon: { label: 'Mixed' },
  mixed: { label: 'Mixed' },
  anime_canon: { label: 'Anime' },
  filler: { label: 'Filler' },
  special: { label: 'Special' },
  unknown: { label: 'Unknown' },
};

export function episodeTypeBadge(type: string | undefined | null): BadgeMeta {
  return episodeTypeMeta[type || 'unknown'] || episodeTypeMeta.unknown;
}

export function runStatusBadge(status: string | undefined | null): BadgeMeta {
  switch (status) {
    case 'success':
      return { label: 'Success' };
    case 'partial':
      return { label: 'Partial' };
    case 'failed':
      return { label: 'Failed' };
    case 'running':
      return { label: 'Running' };
    default:
      return { label: status || 'Unknown' };
  }
}

export function itemStatusBadge(status: string | undefined | null): BadgeMeta {
  switch (status) {
    case 'success':
      return { label: 'Success' };
    case 'failed':
      return { label: 'Failed' };
    default:
      return { label: status || 'Info' };
  }
}

export function syncStateBadge(enabled: boolean): BadgeMeta {
  return enabled
    ? { label: 'Syncing' }
    : { label: 'Paused' };
}

export function monitorStateBadge(
  monitored: boolean,
  yesLabel: string = 'Monitored',
  noLabel: string = 'Unmonitored',
): BadgeMeta {
  return monitored
    ? { label: yesLabel }
    : { label: noLabel };
}
