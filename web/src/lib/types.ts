import type * as v from "valibot";
import type {
  ArtworkProviderSchema,
  EpisodeSchema,
  SettingsSchema,
  ShowSchema,
  SonarrAddOptionsSchema,
  SonarrSeriesSchema,
  SyncLogResponseSchema,
} from "./schemas";

// Shapes the API returns are defined once, by their schemas.
export type Settings = v.InferOutput<typeof SettingsSchema>;
export type Show = v.InferOutput<typeof ShowSchema>;
export type Episode = v.InferOutput<typeof EpisodeSchema>;
export type SonarrSeries = v.InferOutput<typeof SonarrSeriesSchema>;
export type SonarrImage = NonNullable<SonarrSeries["images"]>[number];
export type ArtworkProvider = v.InferOutput<typeof ArtworkProviderSchema>;
export type SyncLogResponse = v.InferOutput<typeof SyncLogResponseSchema>;
export type SyncLog = SyncLogResponse["items"][number];
export type SyncLogItem = NonNullable<SyncLog["items"]>[number];
export type SonarrAddOptions = v.InferOutput<typeof SonarrAddOptionsSchema>;

/** The settings a save may change (the server's settingKeys). Credentials
 *  are write-only: reads report only whether one is set. */
export interface SettingsUpdate {
  sonarr_url?: string;
  sonarr_api_key?: string;
  default_filler_action?: string;
  default_mixed_canon_action?: string;
  default_anime_canon_action?: string;
  sync_interval?: string;
  sonarr_add_root_folder?: string;
  sonarr_add_quality_profile_id?: number;
  sonarr_add_monitor_mode?: string;
  sonarr_add_season_folder?: boolean;
  ntfy_enabled?: boolean;
  ntfy_server_url?: string;
  ntfy_topic?: string;
  ntfy_token?: string;
  ntfy_notify_on?: string;
  ntfy_notify_new_titles?: boolean;
  afl_refresh_interval?: string;
  hold_unclassified?: string;
  artwork_default_source?: string;
  tmdb_api_key?: string;
  fanart_api_key?: string;
  ui_sounds_enabled?: boolean;
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
  /** Explicitly replace an existing mapping for the same series — the
   *  backend returns 409 without it. */
  overwrite?: boolean;
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

export interface ArtworkRefreshResult {
  source: string;
  found: boolean;
  poster_path?: string;
  banner_path?: string;
  logo_path?: string;
}

export interface ArtworkOption {
  url: string;
  thumb_url?: string;
  lang?: string;
  width?: number;
  height?: number;
  likes?: number;
  /** Duplicates of this image that were merged into it. */
  alt_urls?: string[];
}

export interface ArtworkOptions {
  posters: ArtworkOption[];
  banners: ArtworkOption[];
  logos: ArtworkOption[];
}

export interface ArtworkSelectPayload {
  source: string;
  poster_url: string;
  banner_url: string;
  logo_url: string;
}

export interface SelectOption {
  value: string;
  label: string;
}

export interface SonarrAddPayload {
  series: SonarrSeries & { seriesType: string };
  root_folder_path: string;
  quality_profile_id: number;
  monitor_mode: string;
  season_folder: boolean;
  search_on_add: boolean;
}

export interface TestConnectionResult {
  success: boolean;
  error?: string;
}
