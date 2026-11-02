import type { Episode } from "../lib/types";

export const WATERMARK_AUTO = "auto";
export const WATERMARK_MANUAL = "manual";

// Providers that expose many candidate images (the artwork picker).
// Sonarr/anilist/kitsu have a single best pick and no candidates endpoint.
/** Largest image the server accepts for a custom upload. */
export const MAX_CUSTOM_IMAGE_BYTES = 10 << 20;

export const PICKER_SOURCES = new Set(["fanart", "tmdb"]);

export const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean =>
  (action || (fallback ? "monitor" : "unmonitor")) === "monitor";
export const trackedToAction = (tracked: boolean): string => (tracked ? "monitor" : "unmonitor");

function isSpecialSeasonNumber(num: number | undefined): boolean {
  return num === -1 || num === 0;
}
export function seasonLabel(num: number | undefined): string {
  return isSpecialSeasonNumber(num) ? "Specials" : `Season ${num}`;
}
export function displayEpisodeNumber(ep: Episode): number {
  if (isSpecialSeasonNumber(ep.season_number)) return ep.sonarr_episode_number ?? 0;
  return ep.absolute_episode_number ?? ep.episode_num ?? ep.sonarr_episode_number ?? 0;
}
export function episodeLabel(ep: Episode): string {
  return isSpecialSeasonNumber(ep.season_number)
    ? `Special ${displayEpisodeNumber(ep)}`
    : `Episode #${displayEpisodeNumber(ep)}`;
}
export function episodeSeason(ep: Episode): number {
  const raw = ep.season_number;
  // Only treat valid positive integers as real seasons.
  // null/undefined/0/-1 all go to the "Specials" bucket (-1).
  if (typeof raw !== "number" || !Number.isInteger(raw) || raw <= 0) return -1;
  return raw;
}
export function resolveSonarrEpisode(ep: Episode) {
  if (!ep.sonarr_episode_id) return null;
  return { id: ep.sonarr_episode_id, monitored: !!ep.monitored };
}

/** Combined badge info for an episode type — avoids two separate
 *  lookups (episodeTypeBadge + badgeVariantForType) per item. */
export function episodeBadgeInfo(type: string | undefined | null): {
  label: string;
  variant: "neutral" | "blue" | "purple" | "yellow" | "red";
} {
  switch (type) {
    case "canon":
      return { label: "Manga", variant: "blue" };
    case "anime_canon":
      return { label: "Anime", variant: "purple" };
    case "mixed_canon":
    case "mixed":
      return { label: "Mixed", variant: "yellow" };
    case "filler":
      return { label: "Filler", variant: "red" };
    case "special":
      return { label: "Special", variant: "neutral" };
    default:
      return { label: "TBD", variant: "neutral" };
  }
}

/** Composite syncing key — avoids collision when sonarr_episode_id is 0/null. */
export function epSyncKey(groupSeason: number, ep: Episode): string {
  return `${groupSeason}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id ?? "x"}`;
}

export interface SeasonGroup {
  season: number;
  episodes: Episode[];
  index: number;
}
