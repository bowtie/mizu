import { useEffect, useState, useCallback, useRef } from 'react';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { EmptyState } from '@astryxdesign/core/EmptyState';
import { Card } from '@astryxdesign/core/Card';
import { Divider } from '@astryxdesign/core/Divider';
import { TextInput } from '@astryxdesign/core/TextInput';
import { Search } from 'lucide-react';
import { RefreshCw, History, ChevronDown, TriangleAlert } from 'lucide-react';
import { useAppStore } from '../lib/store';
import { getSyncLogs, triggerSync } from '../lib/api';
import type { SyncLog, SyncLogItem } from '../lib/types';
import { runStatusBadge, itemStatusBadge } from '../lib/fox';

const PAGE_SIZE = 8;

const statusFilters = [
  { value: 'all', label: 'All' },
  { value: 'success', label: 'Success' },
  { value: 'partial', label: 'Partial' },
  { value: 'failed', label: 'Failed' },
];

function formatDuration(startedAt: string | null | undefined, completedAt: string | null | undefined): string {
  if (!startedAt || !completedAt) return 'Running';
  const ms = new Date(completedAt).getTime() - new Date(startedAt).getTime();
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
}

function formatTime(dt: string | null | undefined): string {
  if (!dt) return '-';
  return new Date(dt).toLocaleString();
}

function formatTrigger(trigger: string | undefined): string {
  if (!trigger) return '-';
  return trigger === 'manual' ? 'Manual' : 'Scheduled';
}

function humanSummary(log: SyncLog): string {
  const anime = log.shows_processed || 0;
  const episodes = log.episodes_updated || 0;
  return `Checked ${anime} anime, changed ${episodes} ${episodes === 1 ? 'episode' : 'episodes'}.`;
}

function detailLine(item: SyncLogItem): string {
  if (item.summary) return item.summary;
  const changed = item.episodes_changed || 0;
  return changed > 0
    ? `${changed} ${changed === 1 ? 'episode change' : 'episode changes'}.`
    : 'No episode changes.';
}

function hasDetails(log: SyncLog): boolean {
  return !!(log.items?.length || log.errors);
}

function runBadgeVariant(status: string | undefined): 'success' | 'warning' | 'error' | 'neutral' {
  switch (status) {
    case 'success': return 'success';
    case 'partial': return 'warning';
    case 'failed': return 'error';
    default: return 'neutral';
  }
}

function itemBadgeVariant(status: string | undefined): 'success' | 'error' | 'neutral' {
  switch (status) {
    case 'success': return 'success';
    case 'failed': return 'error';
    default: return 'neutral';
  }
}

export function ActivityPage() {
  const { addNotification } = useAppStore();
  const [loading, setLoading] = useState(true);
  const [logs, setLogs] = useState<SyncLog[]>([]);
  const [syncing, setSyncing] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [nextOffset, setNextOffset] = useState(0);
  const [expandedId, setExpandedId] = useState<string | number | null>(null);
  const [statusFilter, setStatusFilter] = useState('all');
  const [searchQuery, setSearchQuery] = useState('');
  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const filteredLogs = logs.filter((log) => {
    if (statusFilter !== 'all' && log.status !== statusFilter) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      const summary = humanSummary(log).toLowerCase();
      const itemTitles = (log.items || []).map((i) => (i.sonarr_title || '').toLowerCase()).join(' ');
      if (!summary.includes(q) && !itemTitles.includes(q) && !(log.errors || '').toLowerCase().includes(q)) return false;
    }
    return true;
  });

  const loadActivity = useCallback(async (reset = true) => {
    if (reset) {
      setLoading(logs.length === 0);
    } else {
      setLoadingMore(true);
    }
    try {
      const page = (await getSyncLogs(PAGE_SIZE, reset ? 0 : nextOffset)) || { items: [], has_more: false, next_offset: 0 };
      const newLogs = page.items || [];
      setLogs(reset ? newLogs : (prev) => [...prev, ...newLogs]);
      setHasMore(!!page.has_more);
      setNextOffset(page.next_offset ?? (reset ? newLogs.length : nextOffset + newLogs.length));
    } catch (err: any) {
      addNotification('Failed to load activity: ' + err.message, 'error');
    }
    setLoading(false);
    setLoadingMore(false);
  }, [nextOffset, logs.length, addNotification]);

  useEffect(() => {
    loadActivity(true);
  }, []);

  useEffect(() => {
    return () => { if (pollTimer.current) clearTimeout(pollTimer.current); };
  }, []);

  const startPolling = () => {
    if (pollTimer.current) clearTimeout(pollTimer.current);
    let attempts = 0;
    const poll = async () => {
      attempts++;
      await loadActivity(true);
      setLogs((current) => {
        if (current.some((l) => l.status === 'running') && attempts < 20) {
          pollTimer.current = setTimeout(poll, 2000);
        }
        return current;
      });
    };
    pollTimer.current = setTimeout(poll, 1800);
  };

  const handleSync = async () => {
    setSyncing(true);
    try {
      await triggerSync();
      addNotification('Sync started', 'info');
      startPolling();
    } catch (err: any) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    setSyncing(false);
  };

  const toggleExpand = (id: string | number) => {
    setExpandedId(expandedId === id ? null : id);
  };

  return (
    <Section padding={6}>
      <VStack gap={6}>
        {/* Header */}
        <HStack justify="between" align="start" wrap="wrap">
          <VStack gap={1}>
            <Heading level={1}>Activity</Heading>
            <Text type="supporting">Sync history and per-anime change logs.</Text>
          </VStack>
          <Button
            label={syncing ? 'Syncing...' : 'Sync now'}
            variant="primary"
            icon={<RefreshCw size={16} />}
            onClick={handleSync}
            isLoading={syncing}
          />
        </HStack>

        {loading ? (
          <VStack gap={3} hAlign="center" padding={10}>
            <Spinner size="lg" label="Loading activity" />
          </VStack>
        ) : logs.length === 0 ? (
          <EmptyState
            title="No sync activity yet"
            description="Run a sync to start building the activity history for your anime library."
            icon={<History size={22} />}
            actions={<Button label="Sync now" variant="primary" icon={<RefreshCw size={16} />} onClick={handleSync} />}
          />
        ) : (
          <>
            {/* Filters */}
            <HStack gap={3} justify="between" wrap="wrap">
              <HStack gap={0}>
                {statusFilters.map((f) => (
                  <Button
                    key={f.value}
                    label={f.label}
                    variant={statusFilter === f.value ? 'primary' : 'secondary'}
                    size="sm"
                    onClick={() => setStatusFilter(f.value)}
                  />
                ))}
              </HStack>
              <TextInput
                label="Search logs"
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Search logs..."
                startIcon={Search}
                isLabelHidden
                width={250}
              />
            </HStack>

            <HStack gap={3} align="center">
              <Text type="supporting">Recent runs</Text>
              <Divider />
            </HStack>

            {filteredLogs.length === 0 ? (
              <EmptyState title="No matching logs" description="Try adjusting your filters or search query." />
            ) : (
              <VStack gap={3}>
                {filteredLogs.map((log) => {
                  const runBadge = runStatusBadge(log.status);
                  return (
                    <Card key={String(log.id)} padding={4}>
                      <VStack gap={2}>
                        <HStack justify="between" align="start" wrap="wrap">
                          <VStack gap={1}>
                            <HStack gap={2} align="center">
                              <Badge variant={runBadgeVariant(log.status)} label={runBadge.label} />
                              <Text type="body" weight="medium">{humanSummary(log)}</Text>
                            </HStack>
                            <Text type="supporting" hasTabularNumbers>
                              {formatTrigger(log.trigger)} · {formatDuration(log.started_at, log.completed_at)} · {formatTime(log.started_at)}
                            </Text>
                          </VStack>
                          {hasDetails(log) && (
                            <Button
                              label="Details"
                              variant="ghost"
                              size="sm"
                              icon={<ChevronDown
                                size={12}
                                style={{ transform: expandedId === log.id ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}
                              />}
                              onClick={() => toggleExpand(log.id)}
                            />
                          )}
                        </HStack>

                        {log.errors && (
                          <HStack gap={2} align="start" style={{ color: 'var(--color-error)' }}>
                            <TriangleAlert size={14} />
                            <Text type="supporting">This run completed with errors that need attention.</Text>
                          </HStack>
                        )}

                        {expandedId === log.id && hasDetails(log) && (
                          <VStack gap={2} padding={3}>
                            {log.items?.length && (
                              <VStack gap={2}>
                                {log.items.map((item, idx) => {
                                  const itemBadge = itemStatusBadge(item.status);
                                  return (
                                    <Section key={idx} variant="muted" padding={3}>
                                      <HStack justify="between" align="start" wrap="wrap">
                                        <VStack gap={0}>
                                          <Text type="body" weight="semibold">{item.sonarr_title}</Text>
                                          <Text type="supporting">{detailLine(item)}</Text>
                                        </VStack>
                                        <Badge variant={itemBadgeVariant(item.status)} label={itemBadge.label} />
                                      </HStack>
                                      {item.error && (
                                        <HStack gap={2} align="start" style={{ color: 'var(--color-error)' }}>
                                          <TriangleAlert size={14} />
                                          <Text type="supporting">{item.error}</Text>
                                        </HStack>
                                      )}
                                    </Section>
                                  );
                                })}
                              </VStack>
                            )}
                            {log.errors && (
                              <Section variant="muted" padding={3}>
                                <HStack gap={2} align="center">
                                  <TriangleAlert size={14} />
                                  <Text type="body" weight="medium">Run errors</Text>
                                </HStack>
                                <Text type="supporting">{log.errors}</Text>
                              </Section>
                            )}
                          </VStack>
                        )}
                      </VStack>
                    </Card>
                  );
                })}
              </VStack>
            )}

            {hasMore && (
              <HStack justify="center">
                <Button
                  label={loadingMore ? 'Loading...' : 'Load more'}
                  variant="secondary"
                  onClick={() => loadActivity(false)}
                  isLoading={loadingMore}
                />
              </HStack>
            )}
          </>
        )}
      </VStack>
    </Section>
  );
}
