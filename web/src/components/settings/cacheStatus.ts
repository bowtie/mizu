import { formatRelativeTime, parseTimestamp } from "../../lib/time";

/** Status line for the AFL show cache row in the Sync settings card. */
export function cacheStatus(
  count: number,
  refreshedAt: string | null | undefined,
  now: Date = new Date(),
): string {
  const date = parseTimestamp(refreshedAt);
  return date
    ? `${count} shows cached, refreshed ${formatRelativeTime(date, now)}.`
    : `${count} shows cached. Never refreshed.`;
}
