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
/**
 * The number line under an episode's title. An episode AnimeFillerList
 * doesn't count (a banned one, say) has no AFL number, and any number
 * shown for it would repeat its neighbour's, so it gets none.
 */
export function episodeNumberText(ep: Episode): string {
  // TVDB's specials sit in their own group; a number there says nothing.
  // (Sonarr's season 0 arrives as no season at all.)
  if (episodeSeason(ep) === -1) return "";
  if (ep.not_on_afl) return "Not listed";
  return `Episode ${displayEpisodeNumber(ep)}`;
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
/** The type badge of an episode row. An episode Mizu holds for
 *  AnimeFillerList shows "Waiting" until it is listed. */
export function episodeBadgeInfo(
  type: string | undefined | null,
  held = false,
  notOnAFL = false,
): {
  label: string;
  variant: "neutral" | "blue" | "purple" | "yellow" | "red";
} {
  if (held) return { label: "Waiting", variant: "neutral" };
  if (notOnAFL) return { label: "—", variant: "neutral" };
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

/** An episode's air date as shown in the list, e.g. "7 Nov 2026". The
 *  short form leaves out the current year, for the narrow phone layout. */
export function formatAirDate(airDate: string | undefined | null, short = false): string {
  if (!airDate) return "";
  const date = new Date(airDate);
  const thisYear = date.getFullYear() === new Date().getFullYear();
  return date.toLocaleDateString(undefined, {
    year: short && thisYear ? undefined : "numeric",
    month: "short",
    day: "numeric",
  });
}
