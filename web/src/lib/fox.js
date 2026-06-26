export const episodeTypeMeta = {
  canon: { label: 'Manga', variant: 'blue' },
  mixed_canon: { label: 'Mixed', variant: 'amber' },
  mixed: { label: 'Mixed', variant: 'amber' },
  anime_canon: { label: 'Anime', variant: 'violet' },
  filler: { label: 'Filler', variant: 'rose' },
  special: { label: 'Special', variant: 'secondary' },
  unknown: { label: 'Unknown', variant: 'secondary' },
};

export function episodeTypeBadge(type) {
  return episodeTypeMeta[type] || episodeTypeMeta.unknown;
}

export function runStatusBadge(status) {
  switch (status) {
    case 'success':
      return { label: 'Success', variant: 'green' };
    case 'partial':
      return { label: 'Partial', variant: 'amber' };
    case 'failed':
      return { label: 'Failed', variant: 'rose' };
    case 'running':
      return { label: 'Running', variant: 'blue' };
    default:
      return { label: status || 'Unknown', variant: 'secondary' };
  }
}

export function itemStatusBadge(status) {
  switch (status) {
    case 'success':
      return { label: 'Success', variant: 'green' };
    case 'failed':
      return { label: 'Failed', variant: 'rose' };
    default:
      return { label: status || 'Info', variant: 'secondary' };
  }
}

export function syncStateBadge(enabled) {
  return enabled
    ? { label: 'Syncing', variant: 'green' }
    : { label: 'Paused', variant: 'secondary' };
}

export function monitorStateBadge(monitored, yesLabel = 'Monitored', noLabel = 'Unmonitored') {
  return monitored
    ? { label: yesLabel, variant: 'green' }
    : { label: noLabel, variant: 'secondary' };
}
