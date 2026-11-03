import type { SyncLog, SyncLogItem } from "./types";

export function formatDuration(
  startedAt: string | null | undefined,
  completedAt: string | null | undefined,
): string {
  if (!startedAt || !completedAt) return "Running";
  const ms = new Date(completedAt).getTime() - new Date(startedAt).getTime();
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
}

export function formatTrigger(trigger: string | undefined): string {
  if (!trigger) return "-";
  if (trigger === "manual") return "Manual";
  if (trigger === "scheduled") return "Scheduled";
  if (trigger === "afl_cache_refresh") return "AFL cache refresh";
  if (trigger === "webhook") return "Sonarr webhook";
  if (trigger === "filler_cleanup") return "Filler cleanup";
  return trigger;
}

export function humanSummary(log: SyncLog): string {
  if (log.trigger === "afl_cache_refresh") {
    const newShows = log.shows_processed || 0;
    return newShows > 0
      ? `${newShows} new anime found in AFL cache`
      : "AFL cache refreshed, no new anime";
  }
  const anime = log.shows_processed || 0;
  const episodes = log.episodes_updated || 0;
  const epLabel = episodes === 1 ? "episode" : "episodes";
  return episodes > 0 ? `${anime} anime, ${episodes} ${epLabel} changed` : `${anime} anime checked`;
}

export function detailLine(item: SyncLogItem): string {
  if (item.summary) return item.summary;
  const changed = item.episodes_changed || 0;
  return changed > 0
    ? `${changed} ${changed === 1 ? "episode change" : "episode changes"}.`
    : "No episode changes.";
}

export function hasDetails(log: SyncLog): boolean {
  return !!(log.items?.length || log.errors);
}
