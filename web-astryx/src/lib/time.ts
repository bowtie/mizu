const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
];

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

/** "3 days ago", "in 2 hours", "just now" — for a timestamp relative to now. */
export function formatRelativeTime(value: string | Date, now: Date = new Date()): string {
  const seconds = Math.round((new Date(value).getTime() - now.getTime()) / 1000);
  const abs = Math.abs(seconds);
  if (abs < 60) return "just now";
  for (const [unit, size] of UNITS) {
    if (abs >= size) return rtf.format(Math.round(seconds / size), unit);
  }
  return "just now";
}

const INTERVAL_MS: Record<string, number> = {
  "1h": 3600_000,
  "3h": 3 * 3600_000,
  "6h": 6 * 3600_000,
  "12h": 12 * 3600_000,
  "24h": 24 * 3600_000,
};

/** A sync is overdue once it is more than two full intervals old. */
export function isSyncOverdue(
  lastSync: string | Date,
  interval: string | undefined,
  now: Date = new Date(),
): boolean {
  const ms = INTERVAL_MS[interval ?? "6h"] ?? INTERVAL_MS["6h"];
  return now.getTime() - new Date(lastSync).getTime() > 2 * ms;
}
