import { useSyncLogs } from "../lib/queries";
import { formatRelativeTime, isSyncOverdue } from "../lib/time";

/** "Last synced 3 hours ago" for the Library header, flagged when overdue. */
export function LastSyncedText({ interval }: { interval?: string }) {
  const { data } = useSyncLogs(10);
  const last = data?.items.find(
    (l) => l.started_at && (l.trigger === "scheduled" || l.trigger === "manual"),
  );
  if (!last?.started_at) return <>Never synced</>;

  const overdue = isSyncOverdue(last.started_at, interval);
  return (
    <span
      title={new Date(last.started_at).toLocaleString()}
      style={overdue ? { color: "var(--color-warning)" } : undefined}
    >
      Last synced {formatRelativeTime(last.started_at)}
      {overdue ? " (overdue)" : ""}
    </span>
  );
}
