import * as v from "valibot";

// ── Settings ──

export const SettingsSchema = v.object({
  sonarr_url: v.string(),
  sonarr_api_key_masked: v.optional(v.string()),
  sonarr_api_key_set: v.optional(v.boolean()),
  default_filler_action: v.string(),
  default_mixed_canon_action: v.string(),
  default_anime_canon_action: v.string(),
  sync_interval: v.string(),
  sonarr_add_root_folder: v.string(),
  sonarr_add_quality_profile_id: v.number(),
  sonarr_add_monitor_mode: v.string(),
  sonarr_add_season_folder: v.boolean(),
  afl_show_cache_refreshed_at: v.union([v.string(), v.null()]),
  afl_show_cache_count: v.number(),
  artwork_default_source: v.optional(v.string()),
  tmdb_api_key_masked: v.optional(v.string()),
  tmdb_api_key_set: v.optional(v.boolean()),
  fanart_api_key_masked: v.optional(v.string()),
  fanart_api_key_set: v.optional(v.boolean()),
  ntfy_enabled: v.optional(v.boolean()),
  ntfy_server_url: v.optional(v.string()),
  ntfy_topic: v.optional(v.string()),
  ntfy_token_set: v.optional(v.boolean()),
  ntfy_notify_on: v.optional(v.picklist(["always", "changes", "failures"])),
  ntfy_notify_new_titles: v.optional(v.boolean()),
  afl_refresh_interval: v.optional(v.picklist(["1d", "7d", "30d", "off"])),
  hold_unclassified: v.optional(v.picklist(["off", "3d", "7d", "14d", "listed"])),
  ui_sounds_enabled: v.optional(v.boolean()),
});

// ── Settings Form (form-field shape, not API response) ──

export const SettingsFormSchema = v.object({
  sonarrUrl: v.pipe(
    v.string(),
    v.check(
      (input) => !input || /^https?:\/\/.+/.test(input),
      "URL must start with http:// or https://",
    ),
  ),
  sonarrApiKey: v.string(),
  syncInterval: v.string(),
  aflRefreshInterval: v.string(),
  holdUnclassified: v.string(),
  artworkDefaultSource: v.string(),
  tmdbApiKey: v.string(),
  fanartApiKey: v.string(),
  trackFillerDefault: v.boolean(),
  trackMixedDefault: v.boolean(),
  trackAnimeCanonDefault: v.boolean(),
  ntfyEnabled: v.boolean(),
  ntfyServerUrl: v.string(),
  ntfyTopic: v.string(),
  ntfyToken: v.string(),
  notifyOn: v.picklist(["always", "changes", "failures"]),
  notifyNewTitles: v.boolean(),
  soundsEnabled: v.boolean(),
});

export type SettingsForm = v.InferOutput<typeof SettingsFormSchema>;

// ── Show ──

export const ShowSchema = v.object({
  id: v.union([v.number(), v.string()]),
  sonarr_series_id: v.number(),
  sonarr_title: v.string(),
  afl_title: v.optional(v.string()),
  afl_slug: v.optional(v.string()),
  artwork_source: v.optional(v.string()),
  artwork_poster_path: v.optional(v.string()),
  artwork_banner_path: v.optional(v.string()),
  artwork_logo_path: v.optional(v.string()),
  artwork_poster_url: v.optional(v.string()),
  artwork_banner_url: v.optional(v.string()),
  artwork_logo_url: v.optional(v.string()),
  overview: v.optional(v.string()),
  poster: v.optional(v.string()),
  sync_watermark: v.number(),
  enabled: v.boolean(),
  filler_action: v.string(),
  mixed_action: v.string(),
  anime_canon_action: v.string(),
  watermark_mode: v.string(),
  notifications_muted: v.optional(v.boolean()),
  artwork_custom_slots: v.optional(v.array(v.string())),
});

// ── Episode ──

export const EpisodeSchema = v.object({
  id: v.optional(v.number()),
  sonarr_episode_id: v.optional(v.number()),
  episode_num: v.optional(v.number()),
  afl_episode_number: v.optional(v.number()),
  season_number: v.optional(v.number()),
  sonarr_episode_number: v.optional(v.number()),
  absolute_episode_number: v.optional(v.number()),
  title: v.optional(v.string()),
  type: v.optional(v.string()),
  monitored: v.optional(v.boolean()),
  air_date: v.optional(v.string()),
  /** Waits unmonitored for AnimeFillerList to list it. */
  held: v.optional(v.boolean()),
});

// ── SonarrSeries ──

const SonarrImageSchema = v.object({
  url: v.optional(v.string()),
  remoteUrl: v.optional(v.string()),
  coverType: v.optional(v.string()),
});

export const SonarrSeriesSchema = v.object({
  id: v.number(),
  title: v.string(),
  slug: v.optional(v.string()),
  year: v.optional(v.number()),
  status: v.optional(v.string()),
  overview: v.optional(v.string()),
  description: v.optional(v.string()),
  seriesType: v.optional(v.string()),
  monitored: v.optional(v.boolean()),
  rootFolderPath: v.optional(v.string()),
  qualityProfileId: v.optional(v.number()),
  images: v.optional(v.array(SonarrImageSchema)),
  tvdbId: v.optional(v.number()),
  network: v.optional(v.string()),
  statistics: v.optional(v.object({ episodeCount: v.optional(v.number()) })),
  manualMatchPending: v.optional(v.boolean()),
});

// ── ArtworkProvider ──

export const ArtworkProviderSchema = v.object({
  id: v.string(),
  label: v.string(),
  needs_key: v.boolean(),
  available: v.boolean(),
});

// ── SyncLog ──

const SyncLogItemSchema = v.object({
  id: v.optional(v.number()),
  sync_log_id: v.optional(v.number()),
  mapping_id: v.optional(v.number()),
  status: v.optional(v.string()),
  sonarr_title: v.optional(v.string()),
  summary: v.optional(v.string()),
  episodes_changed: v.optional(v.number()),
  error: v.optional(v.string()),
});

const SyncLogSchema = v.object({
  id: v.union([v.number(), v.string()]),
  status: v.optional(v.string()),
  trigger: v.optional(v.string()),
  started_at: v.optional(v.string()),
  completed_at: v.optional(v.string()),
  shows_processed: v.optional(v.number()),
  episodes_updated: v.optional(v.number()),
  items: v.optional(v.array(SyncLogItemSchema)),
  errors: v.optional(v.string()),
});

export const SyncLogResponseSchema = v.object({
  items: v.array(SyncLogSchema),
  has_more: v.optional(v.boolean()),
  next_offset: v.optional(v.number()),
});

// ── Misc ──

export const SonarrAddOptionsSchema = v.object({
  root_folders: v.array(v.object({ path: v.string() })),
  quality_profiles: v.array(v.object({ id: v.number(), name: v.string() })),
});

// ── Inferred types (replace the manual interfaces in types.ts over time) ──

export type Show = v.InferOutput<typeof ShowSchema>;
export type Episode = v.InferOutput<typeof EpisodeSchema>;
export type SonarrSeries = v.InferOutput<typeof SonarrSeriesSchema>;
