export interface Notification {
  id: number;
  message: string;
  type: string;
  duration: number;
}

export interface Settings {
  sonarr_url: string;
  sonarr_api_key: string;
  sonarr_api_key_masked?: string;
  sonarr_api_key_set?: boolean;
  default_filler_action: string;
  default_mixed_canon_action: string;
  default_anime_canon_action: string;
  sync_interval: string;
  sonarr_add_root_folder: string;
  sonarr_add_quality_profile_id: number;
  sonarr_add_monitor_mode: string;
  sonarr_add_season_folder: boolean;
  afl_show_cache_refreshed_at: string | null;
  afl_show_cache_count: number;
  artwork_default_source?: string;
  tmdb_api_key?: string;
  tmdb_api_key_set?: boolean;
  fanart_api_key?: string;
  fanart_api_key_set?: boolean;
  ntfy_enabled?: boolean;
  ntfy_server_url?: string;
  ntfy_topic?: string;
  ntfy_token_set?: boolean;
  [key: string]: unknown;
}

export interface SonarrImage {
  url?: string;
  remoteUrl?: string;
  coverType?: string;
}

export interface SonarrSeries {
  id: number;
  title: string;
  slug?: string;
  year?: number;
  status?: string;
  overview?: string;
  description?: string;
  seriesType?: string;
  monitored?: boolean;
  rootFolderPath?: string;
  qualityProfileId?: number;
  images?: SonarrImage[];
  tvdbId?: number;
  network?: string;
  statistics?: {
    episodeCount?: number;
  };
  manualMatchPending?: boolean;
  [key: string]: unknown;
}

export interface Show {
  id: number | string;
  sonarr_series_id: number;
  sonarr_title: string;
  afl_title?: string;
  afl_slug?: string;
  artwork_source?: string;
  artwork_poster_path?: string;
  artwork_banner_path?: string;
  artwork_logo_path?: string;
  poster?: string;
  sync_watermark: number;
  enabled: boolean;
  filler_action: string;
  mixed_action: string;
  anime_canon_action: string;
  watermark_mode: string;
  [key: string]: unknown;
}

export type ShowUpdate = Partial<Show>;

export interface MappingCreate {
  sonarr_series_id?: number;
  sonarr_title?: string;
  afl_slug?: string;
  afl_title?: string;
  filler_action?: string;
  mixed_action?: string;
  anime_canon_action?: string;
  start_mode?: string;
  [key: string]: unknown;
}

export interface Episode {
  id?: number;
  sonarr_episode_id?: number;
  episode_num?: number;
  afl_episode_number?: number;
  season_number?: number;
  sonarr_episode_number?: number;
  absolute_episode_number?: number;
  title?: string;
  type?: string;
  monitored?: boolean;
  [key: string]: unknown;
}

export interface AFLMatch {
  slug?: string;
  title?: string;
  method?: string;
  confidence?: number;
  afl_show?: {
    slug?: string;
    title?: string;
  };
  [key: string]: unknown;
}

export interface ArtworkProvider {
  id: string;
  label: string;
  needs_key: boolean;
  available: boolean;
}

export interface ArtworkRefreshResult {
  source: string;
  found: boolean;
  poster_path?: string;
  banner_path?: string;
  logo_path?: string;
}

export interface SelectOption {
  value: string;
  label: string;
}

export interface SonarrAddOptions {
  root_folders: { path: string }[];
  quality_profiles: { id: number; name: string }[];
}

export interface SonarrAddPayload {
  series: SonarrSeries & { seriesType: string };
  root_folder_path: string;
  quality_profile_id: number;
  monitor_mode: string;
  season_folder: boolean;
  search_on_add: boolean;
}

export interface SyncLogItem {
  id?: number;
  sync_log_id?: number;
  mapping_id?: number;
  status?: string;
  sonarr_title?: string;
  summary?: string;
  episodes_changed?: number;
  error?: string;
}

export interface SyncLog {
  id: number | string;
  status?: string;
  trigger?: string;
  started_at?: string;
  completed_at?: string;
  shows_processed?: number;
  episodes_updated?: number;
  items?: SyncLogItem[];
  errors?: string;
}

export interface SyncLogResponse {
  items: SyncLog[];
  has_more?: boolean;
  next_offset?: number;
}

export interface TestConnectionResult {
  success: boolean;
  error?: string;
}

export interface StoredApiKeyResponse {
  sonarr_api_key?: string;
  sonarr_api_key_set?: boolean;
}
