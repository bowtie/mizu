import { writable } from 'svelte/store';

export const shows = writable([]);
export const syncStatus = writable({
  connected: false,
  aflReachable: false,
  lastSync: null,
  nextSync: null,
  syncing: false,
});
export const settings = writable({
  sonarr_url: '',
  sonarr_api_key: '',
  default_filler_action: 'unmonitor',
  default_mixed_canon_action: 'monitor',
  default_anime_canon_action: 'monitor',
  sync_interval: '6h',
  enable_tagging: false,
  filler_tag_name: 'filler',
  afl_show_cache_refreshed_at: null,
  afl_show_cache_count: 0,
});
export const notifications = writable([]);

let notifId = 0;

export function addNotification(message, type = 'info', duration = 4000) {
  const id = ++notifId;
  notifications.update((n) => [...n, { id, message, type, duration }]);
}

export function removeNotification(id) {
  notifications.update((n) => n.filter((x) => x.id !== id));
}
