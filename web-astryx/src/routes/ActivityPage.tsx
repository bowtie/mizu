import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { Card } from "@astryxdesign/core/Card";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Timestamp } from "@astryxdesign/core/Timestamp";
import { Search } from "lucide-react";
import {
  RefreshCw,
  History,
  ChevronDown,
  TriangleAlert,
  Check,
  CircleAlert,
  Loader2,
  CircleDot,
} from "lucide-react";
import { SegmentedControl, SegmentedControlItem } from "@astryxdesign/core/SegmentedControl";
import { useNotificationStore } from "../lib/store";
import { useTriggerSync } from "../lib/queries";
import { getSyncLogs, errorMessage } from "../lib/api";
import type { SyncLog } from "../lib/types";
import {
  formatDuration,
  formatTrigger,
  humanSummary,
  detailLine,
  hasDetails,
} from "../lib/activity-utils";

const PAGE_SIZE = 8;

// Sync-polling cadence for the Activity page. While the latest sync log is
// "running", poll every 2s for the first ACTIVITY_FAST_POLL_ATTEMPTS polls,
// then fall back to a slow 30s watchdog so a long or hung sync keeps being
// tracked — the page never freezes on a stale "Running" log — without
// runaway polling.
const ACTIVITY_POLL_FIRST_DELAY_MS = 1800;
const ACTIVITY_POLL_INTERVAL_MS = 2000;
const ACTIVITY_FAST_POLL_ATTEMPTS = 150; // ≈ 5 minutes at the 2s cadence
const ACTIVITY_SLOW_POLL_INTERVAL_MS = 30_000;

function statusIcon(status: string | undefined | null) {
  switch (status) {
    case "success":
      return { icon: <Check size={14} />, className: "activity-status-icon-success" };
    case "partial":
      return { icon: <TriangleAlert size={14} />, className: "activity-status-icon-partial" };
    case "failed":
      return { icon: <CircleAlert size={14} />, className: "activity-status-icon-failed" };
    case "running":
      return {
        icon: <Loader2 size={14} className="spin" />,
        className: "activity-status-icon-running",
      };
    default:
      return { icon: <CircleDot size={14} />, className: "activity-status-icon-neutral" };
  }
}

function itemStatusIcon(status: string | undefined | null) {
  switch (status) {
    case "success":
      return { icon: <Check size={12} />, className: "activity-status-icon-success" };
    case "failed":
      return { icon: <CircleAlert size={12} />, className: "activity-status-icon-failed" };
    case "new_in_cache":
      return { icon: <CircleDot size={12} />, className: "activity-status-icon-partial" };
    default:
      return { icon: <CircleDot size={12} />, className: "activity-status-icon-neutral" };
  }
}

export function ActivityPage() {
  const addNotification = useNotificationStore((s) => s.addNotification);
  const triggerSyncMutation = useTriggerSync();
  const [loading, setLoading] = useState(true);
  const [logs, setLogs] = useState<SyncLog[]>([]);
  const [syncing, setSyncing] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [nextOffset, setNextOffset] = useState(0);
  const [expandedId, setExpandedId] = useState<string | number | null>(null);
  const [statusFilter, setStatusFilter] = useState("all");
  const [searchQuery, setSearchQuery] = useState("");
  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pollChainId = useRef(0);
  const isInitialLoad = useRef(true);
  const isMounted = useRef(true);
  const logsRef = useRef<SyncLog[]>([]);

  const filteredLogs = useMemo(
    () =>
      logs.filter((log) => {
        if (statusFilter !== "all" && log.status !== statusFilter) return false;
        if (searchQuery) {
          const q = searchQuery.toLowerCase();
          const summary = humanSummary(log).toLowerCase();
          const itemTitles = (log.items || [])
            .map((i) => (i.sonarr_title || "").toLowerCase())
            .join(" ");
          if (
            !summary.includes(q) &&
            !itemTitles.includes(q) &&
            !(log.errors || "").toLowerCase().includes(q)
          )
            return false;
        }
        return true;
      }),
    [logs, statusFilter, searchQuery],
  );

  const loadActivity = useCallback(
    async (reset = true) => {
      if (reset) {
        setLoading(isInitialLoad.current);
      } else {
        setLoadingMore(true);
      }
      try {
        const offset = reset ? 0 : nextOffset;
        const page = (await getSyncLogs(PAGE_SIZE, offset)) || {
          items: [],
          has_more: false,
          next_offset: 0,
        };
        const newLogs = page.items || [];
        const updatedLogs = reset ? newLogs : [...logsRef.current, ...newLogs];
        logsRef.current = updatedLogs;
        setLogs(updatedLogs);
        setHasMore(!!page.has_more);
        setNextOffset(page.next_offset ?? (reset ? newLogs.length : offset + newLogs.length));
      } catch (err: unknown) {
        addNotification("Failed to load activity: " + errorMessage(err), "error");
      }
      setLoading(false);
      setLoadingMore(false);
      isInitialLoad.current = false;
    },
    [nextOffset, addNotification],
  );

  useEffect(() => {
    loadActivity(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // StrictMode in dev mounts, cleans up, and mounts again; the cleanup
    // below sets isMounted=false, so reset it here or polling would bail
    // forever after the simulated unmount.
    isMounted.current = true;
    return () => {
      isMounted.current = false;
      if (pollTimer.current) clearTimeout(pollTimer.current);
    };
  }, []);

  const startPolling = useCallback(() => {
    if (pollTimer.current) clearTimeout(pollTimer.current);
    // Supersede any earlier chain: a poll that is mid-flight when a newer
    // startPolling runs (e.g. the focus handler while a poll is in the
    // network await) must not keep rescheduling itself alongside the new
    // chain — clearing the pending timer can't stop an in-flight poll.
    const chainId = ++pollChainId.current;
    let attempts = 0;
    const poll = async () => {
      if (!isMounted.current) return;
      attempts++;
      await loadActivity(true);
      if (!isMounted.current) return;
      if (pollChainId.current !== chainId) return; // superseded by a newer chain
      const hasRunning = logsRef.current.some((l) => l.status === "running");
      // Stop once the sync reaches a terminal state (success/partial/failed).
      if (!hasRunning) return;
      // Fast 2s cadence for the first ACTIVITY_FAST_POLL_ATTEMPTS polls, then
      // a slow 30s watchdog — bounded and cheap, but the page never freezes
      // on a stale "Running" log even for a very long or hung sync.
      const delay =
        attempts <= ACTIVITY_FAST_POLL_ATTEMPTS
          ? ACTIVITY_POLL_INTERVAL_MS
          : ACTIVITY_SLOW_POLL_INTERVAL_MS;
      pollTimer.current = setTimeout(poll, delay);
    };
    pollTimer.current = setTimeout(poll, ACTIVITY_POLL_FIRST_DELAY_MS);
  }, [loadActivity]);

  // Refresh when the tab regains focus/visibility: browsers throttle
  // background timers, and a scheduled sync may have started (or a manual
  // sync finished) while the page was hidden — reload immediately and
  // resume polling if a sync is still running. The running-log check must
  // wait for the refetch: logsRef only updates once loadActivity resolves,
  // so a synchronous check read the pre-refresh snapshot and never resumed
  // polling for a sync that started while hidden.
  useEffect(() => {
    let generation = 0;
    const onVisible = async () => {
      if (!isMounted.current) return;
      if (document.visibilityState !== "visible") return;
      const gen = ++generation;
      await loadActivity(true);
      if (!isMounted.current || gen !== generation) return;
      if (logsRef.current.some((l) => l.status === "running")) {
        startPolling();
      }
    };
    window.addEventListener("focus", onVisible);
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.removeEventListener("focus", onVisible);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [loadActivity, startPolling]);

  const handleSync = async () => {
    setSyncing(true);
    try {
      await triggerSyncMutation.mutateAsync();
      addNotification("Sync started", "info");
      startPolling();
    } catch (err: unknown) {
      addNotification("Failed to trigger sync: " + errorMessage(err), "error");
    }
    setSyncing(false);
  };

  const toggleExpand = (id: string | number) => {
    setExpandedId(expandedId === id ? null : id);
  };

  return (
    <Section padding={4} maxWidth={900} className="section-centered page-enter">
      <VStack gap={6}>
        {/* Header */}
        <HStack justify="between" align="center" wrap="wrap" gap={3}>
          <VStack gap={1}>
            <Heading level={1}>Activity</Heading>
            <Text type="supporting" color="secondary">
              Sync history and per-anime change logs.
            </Text>
          </VStack>
          <Button
            label={syncing ? "Syncing..." : "Sync now"}
            variant="primary"
            icon={<RefreshCw size={16} />}
            onClick={handleSync}
            isLoading={syncing}
            data-cuelume-press
          />
        </HStack>

        {loading ? (
          <VStack gap={3}>
            {Array.from({ length: 5 }).map((_, i) => (
              <div
                key={i}
                className="activity-log-card skeleton-card"
                style={{
                  padding: "var(--spacing-4)",
                  display: "flex",
                  flexDirection: "column",
                  gap: "var(--spacing-2)",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
                  <div
                    className="skeleton"
                    style={{ width: 28, height: 28, borderRadius: "50%" }}
                  />
                  <div className="skeleton-text" style={{ width: 140 }} />
                  <div style={{ flex: 1 }} />
                  <div className="skeleton-text" style={{ width: 80 }} />
                </div>
                <div className="skeleton-text" style={{ width: "60%", marginLeft: 40 }} />
              </div>
            ))}
          </VStack>
        ) : logs.length === 0 ? (
          <EmptyState
            title="No sync activity yet"
            description="Run a sync to start building the activity history for your anime library."
            icon={<Icon icon={History} size="lg" color="secondary" />}
            actions={
              <Button
                label={syncing ? "Syncing..." : "Sync now"}
                variant="primary"
                icon={<RefreshCw size={16} />}
                onClick={handleSync}
                isLoading={syncing}
                data-cuelume-press
              />
            }
          />
        ) : (
          <>
            {/* Filters */}
            <VStack gap={3} width="100%">
              <SegmentedControl
                value={statusFilter}
                onChange={setStatusFilter}
                label="Status filter"
                layout="fill"
                size="md"
              >
                <SegmentedControlItem value="all" label="All" />
                <SegmentedControlItem value="running" label="Running" />
                <SegmentedControlItem value="success" label="Success" />
                <SegmentedControlItem value="partial" label="Partial" />
                <SegmentedControlItem value="failed" label="Failed" />
              </SegmentedControl>
              <TextInput
                label="Search logs"
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Search logs..."
                startIcon={Search}
                isLabelHidden
                width="100%"
              />
            </VStack>

            {filteredLogs.length === 0 ? (
              <EmptyState
                title="No matching logs"
                description="Try adjusting your filters or search query."
                icon={<Icon icon={Search} size="lg" color="secondary" />}
                actions={
                  <Button
                    label="Clear filters"
                    variant="secondary"
                    onClick={() => {
                      setStatusFilter("all");
                      setSearchQuery("");
                    }}
                    data-cuelume-press
                  />
                }
              />
            ) : (
              <VStack gap={3}>
                {filteredLogs.map((log) => {
                  const isExpanded = expandedId === log.id;
                  return (
                    <Card
                      key={String(log.id)}
                      padding={3}
                      variant="muted"
                      className={`activity-log-card${isExpanded ? " activity-log-card-expanded" : ""}`}
                    >
                      <VStack gap={2}>
                        <HStack gap={3} align="center" wrap="nowrap" width="100%">
                          <div
                            className={`activity-status-icon ${statusIcon(log.status).className}`}
                          >
                            {statusIcon(log.status).icon}
                          </div>
                          <HStack
                            justify="between"
                            align="center"
                            wrap="wrap"
                            gap={2}
                            className="activity-log-body"
                          >
                            <VStack
                              gap={1}
                              className="activity-log-summary"
                              onClick={hasDetails(log) ? () => toggleExpand(log.id) : undefined}
                              role={hasDetails(log) ? "button" : undefined}
                              tabIndex={hasDetails(log) ? 0 : undefined}
                              onKeyDown={
                                hasDetails(log)
                                  ? (e) => {
                                      if (e.key === "Enter" || e.key === " ") {
                                        e.preventDefault();
                                        toggleExpand(log.id);
                                      }
                                    }
                                  : undefined
                              }
                            >
                              <Text type="body" weight="medium">
                                {humanSummary(log)}
                              </Text>
                              <Text type="supporting" hasTabularNumbers size="sm">
                                {formatTrigger(log.trigger)}{" "}
                                {formatDuration(log.started_at, log.completed_at)}{" "}
                                <Timestamp
                                  value={log.started_at || ""}
                                  format="auto"
                                  type="supporting"
                                  color="secondary"
                                />
                              </Text>
                            </VStack>
                            {hasDetails(log) && (
                              <Button
                                label="Details"
                                variant="ghost"
                                size="sm"
                                icon={
                                  <ChevronDown
                                    size={12}
                                    className={`button-icon-chevron${isExpanded ? " chevron-expanded" : ""}`}
                                  />
                                }
                                onClick={() => toggleExpand(log.id)}
                              />
                            )}
                          </HStack>
                        </HStack>

                        {/* Run-level errors shown only when not expanded (expanded view shows them below) */}
                        {log.errors && !isExpanded && (
                          <HStack gap={2} align="start" className="activity-error-row">
                            <TriangleAlert size={14} />
                            <Text type="supporting">
                              This run completed with errors that need attention.
                            </Text>
                          </HStack>
                        )}

                        {isExpanded && hasDetails(log) && (
                          <VStack gap={2} className="activity-details-expanded">
                            {!!log.items?.length && (
                              <VStack gap={2}>
                                {log.items.map((item, idx) => {
                                  const si = itemStatusIcon(item.status);
                                  return (
                                    <Card key={idx} padding={2} variant="default">
                                      <HStack justify="between" align="center" wrap="wrap" gap={2}>
                                        <VStack gap={0} className="activity-item-detail">
                                          <Text type="body" weight="semibold">
                                            {item.sonarr_title}
                                          </Text>
                                          <Text type="supporting">{detailLine(item)}</Text>
                                          {item.error && (
                                            <HStack
                                              gap={2}
                                              align="start"
                                              className="activity-error-row"
                                            >
                                              <TriangleAlert size={14} />
                                              <Text type="supporting">{item.error}</Text>
                                            </HStack>
                                          )}
                                        </VStack>
                                        <div
                                          className={`activity-status-icon activity-status-icon-sm ${si.className}`}
                                        >
                                          {si.icon}
                                        </div>
                                      </HStack>
                                    </Card>
                                  );
                                })}
                              </VStack>
                            )}
                            {log.errors && (
                              <Card padding={2} variant="default">
                                <HStack gap={2} align="center">
                                  <TriangleAlert size={14} />
                                  <Text type="body" weight="medium">
                                    Run errors
                                  </Text>
                                </HStack>
                                <Text type="supporting">{log.errors}</Text>
                              </Card>
                            )}
                          </VStack>
                        )}
                      </VStack>
                    </Card>
                  );
                })}
              </VStack>
            )}

            {hasMore && filteredLogs.length > 0 && (
              <HStack justify="center" className="load-more-bar">
                <Button
                  label={loadingMore ? "Loading..." : "Load more"}
                  variant="secondary"
                  onClick={() => loadActivity(false)}
                  isLoading={loadingMore}
                  isDisabled={loadingMore}
                  data-cuelume-press
                />
              </HStack>
            )}
          </>
        )}
      </VStack>
    </Section>
  );
}
