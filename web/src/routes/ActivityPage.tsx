import { useEffect, useMemo, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
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
import { queryKeys, useSyncLogPages, useTriggerSync } from "../lib/queries";
import { errorMessage } from "../lib/api";
import {
  formatDuration,
  formatTrigger,
  humanSummary,
  detailLine,
  hasDetails,
} from "../lib/activity-utils";

const PAGE_SIZE = 8;

// After "Sync now", keep polling this long even before the run's log row
// appears (the server creates it just after accepting the trigger).
const SYNC_START_GRACE_MS = 10_000;

function statusIcon(status: string | undefined | null) {
  switch (status) {
    case "success":
      return { icon: <Check size={18} />, className: "activity-status-icon-success" };
    case "partial":
      return { icon: <TriangleAlert size={18} />, className: "activity-status-icon-partial" };
    case "failed":
      return { icon: <CircleAlert size={18} />, className: "activity-status-icon-failed" };
    case "running":
      return {
        icon: <Loader2 size={18} className="spin" />,
        className: "activity-status-icon-running",
      };
    default:
      return { icon: <CircleDot size={18} />, className: "activity-status-icon-neutral" };
  }
}

function itemStatusIcon(status: string | undefined | null) {
  switch (status) {
    case "success":
      return { icon: <Check size={14} />, className: "activity-status-icon-success" };
    case "failed":
      return { icon: <CircleAlert size={14} />, className: "activity-status-icon-failed" };
    case "new_in_cache":
      return { icon: <CircleDot size={14} />, className: "activity-status-icon-partial" };
    default:
      return { icon: <CircleDot size={14} />, className: "activity-status-icon-neutral" };
  }
}

export function ActivityPage() {
  const addNotification = useNotificationStore((s) => s.addNotification);
  const triggerSyncMutation = useTriggerSync();
  const queryClient = useQueryClient();
  const [pollUntil, setPollUntil] = useState(0);
  const logsQuery = useSyncLogPages(PAGE_SIZE, pollUntil);
  const logs = useMemo(
    () => logsQuery.data?.pages.flatMap((p) => p.items ?? []) ?? [],
    [logsQuery.data],
  );
  const loading = logsQuery.isPending;
  const [syncing, setSyncing] = useState(false);
  const [expandedId, setExpandedId] = useState<string | number | null>(null);
  const [statusFilter, setStatusFilter] = useState("all");
  const [searchQuery, setSearchQuery] = useState("");

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

  const loadError = logsQuery.error;
  useEffect(() => {
    if (loadError) addNotification("Couldn't load activity: " + errorMessage(loadError), "error");
  }, [loadError, addNotification]);

  // When a run finishes, everything it touched is stale: the library, the
  // episode lists and the "last synced" line.
  const anyRunning = logs.some((l) => l.status === "running");
  const wasRunning = useRef(anyRunning);
  useEffect(() => {
    if (wasRunning.current && !anyRunning) {
      void queryClient.invalidateQueries({ queryKey: ["syncLogs"] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.shows });
      void queryClient.invalidateQueries({ queryKey: ["episodes"] });
    }
    wasRunning.current = anyRunning;
  }, [anyRunning, queryClient]);

  const handleSync = async () => {
    setSyncing(true);
    try {
      await triggerSyncMutation.mutateAsync();
      addNotification("Sync started", "info");
      setPollUntil(Date.now() + SYNC_START_GRACE_MS);
    } catch (err: unknown) {
      addNotification("Couldn't start the sync: " + errorMessage(err), "error");
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
            description="Syncs show up here."
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
                description="Try another filter or search."
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
                                {[
                                  formatTrigger(log.trigger),
                                  formatDuration(log.started_at, log.completed_at),
                                ]
                                  .filter(Boolean)
                                  .join(" · ")}
                                {" · "}
                                <Timestamp
                                  value={log.started_at || ""}
                                  format="relative"
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
                          <HStack
                            gap={2}
                            align="center"
                            className="activity-error-row activity-error-row-center"
                          >
                            <TriangleAlert size={14} />
                            <Text type="supporting">This run finished with errors.</Text>
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

            {logsQuery.hasNextPage && filteredLogs.length > 0 && (
              <HStack justify="center" className="load-more-bar">
                <Button
                  label={logsQuery.isFetchingNextPage ? "Loading..." : "Load more"}
                  variant="secondary"
                  onClick={() => void logsQuery.fetchNextPage()}
                  isLoading={logsQuery.isFetchingNextPage}
                  isDisabled={logsQuery.isFetchingNextPage}
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
