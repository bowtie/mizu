import { writable, type Writable } from 'svelte/store';
import type { Notification, Settings, Show, SyncStatus } from './types.ts';

export const shows: Writable<Show[]> = writable([]);

export const syncStatus: Writable<SyncStatus> = writable({
  connected: false,
  aflReachable: false,
  lastSync: null,
  nextSync: null,
  syncing: false,
});

export const settings: Writable<Settings> = writable({
  sonarr_url: '',
  sonarr_api_key: '',
  default_filler_action: 'unmonitor',
  default_mixed_canon_action: 'monitor',
  default_anime_canon_action: 'monitor',
  sync_interval: '6h',
  enable_anilist_artwork: false,
  sonarr_add_root_folder: '',
  sonarr_add_quality_profile_id: 0,
  sonarr_add_monitor_mode: 'all',
  sonarr_add_season_folder: true,
  afl_show_cache_refreshed_at: null,
  afl_show_cache_count: 0,
});

export const notifications: Writable<Notification[]> = writable([]);

let notifId = 0;

export function addNotification(message: string, type: string = 'info', duration: number = 4000): void {
  const id = ++notifId;
  notifications.update((n) => [...n, { id, message, type, duration }]);
}

export function removeNotification(id: number): void {
  notifications.update((n) => n.filter((x) => x.id !== id));
}
