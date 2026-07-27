import { useEffect, useState, useCallback, useRef, useMemo } from 'react';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { EmptyState } from '@astryxdesign/core/EmptyState';
import { Icon } from '@astryxdesign/core/Icon';
import { Card } from '@astryxdesign/core/Card';
import { TextInput } from '@astryxdesign/core/TextInput';
import { Timestamp } from '@astryxdesign/core/Timestamp';
import { Search } from 'lucide-react';
import { RefreshCw, History, ChevronDown, TriangleAlert } from 'lucide-react';
import { useAppStore } from '../lib/store';
import { getSyncLogs, triggerSync } from '../lib/api';
import type { SyncLog } from '../lib/types';
import { runStatusBadge, itemStatusBadge, runStatusBadgeVariant, itemStatusBadgeVariant } from '../lib/fox';
import { formatDuration, formatTrigger, humanSummary, detailLine, hasDetails } from '../lib/activity-utils';

const PAGE_SIZE = 8;

const statusFilters = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', color: 'var(--color-accent)' },
  { value: 'success', label: 'Success', color: 'var(--color-success)' },
  { value: 'partial', label: 'Partial', color: 'var(--color-warning)' },
  { value: 'failed', label: 'Failed', color: 'var(--color-error)' },
];

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
  const isInitialLoad = useRef(true);

  const filteredLogs = useMemo(() => logs.filter((log) => {
    if (statusFilter !== 'all' && log.status !== statusFilter) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      const summary = humanSummary(log).toLowerCase();
      const itemTitles = (log.items || []).map((i) => (i.sonarr_title || '').toLowerCase()).join(' ');
      if (!summary.includes(q) && !itemTitles.includes(q) && !(log.errors || '').toLowerCase().includes(q)) return false;
    }
    return true;
  }), [logs, statusFilter, searchQuery]);

  const loadActivity = useCallback(async (reset = true) => {
    if (reset) {
      setLoading(isInitialLoad.current);
    } else {
      setLoadingMore(true);
    }
    try {
      const offset = reset ? 0 : nextOffset;
      const page = (await getSyncLogs(PAGE_SIZE, offset)) || { items: [], has_more: false, next_offset: 0 };
      const newLogs = page.items || [];
      setLogs(reset ? newLogs : (prev) => [...prev, ...newLogs]);
      setHasMore(!!page.has_more);
      setNextOffset(page.next_offset ?? (reset ? newLogs.length : offset + newLogs.length));
    } catch (err: any) {
      addNotification('Failed to load activity: ' + err.message, 'error');
    }
    setLoading(false);
    setLoadingMore(false);
    isInitialLoad.current = false;
  }, [nextOffset, addNotification]);

  useEffect(() => {
    loadActivity(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
    <Section padding={4} maxWidth={900} style={{ marginInline: 'auto' }}>
      <VStack gap={6}>
        {/* Header */}
        <HStack justify="between" align="center" wrap="wrap" gap={3}>
          <VStack gap={1}>
            <Heading level={1}>Activity</Heading>
            <Text type="supporting" color="secondary">Sync history and per-anime change logs.</Text>
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
          <VStack gap={3} hAlign="center" padding={6}>
            <Spinner size="lg" label="Loading activity" />
          </VStack>
        ) : logs.length === 0 ? (
          <EmptyState
            title="No sync activity yet"
            description="Run a sync to start building the activity history for your anime library."
            icon={<Icon icon={History} size="lg" color="secondary" />}
            actions={<Button label="Sync now" variant="primary" icon={<RefreshCw size={16} />} onClick={handleSync} />}
          />
        ) : (
          <>
            {/* Filters */}
            <VStack gap={3} style={{ width: '100%' }}>
              <HStack gap={2} wrap="wrap" align="center">
                {statusFilters.map((f) => {
                  const isActive = statusFilter === f.value;
                  return (
                    <Button
                      key={f.value}
                      label={f.label}
                      variant={isActive ? 'primary' : 'secondary'}
                      size="sm"
                      style={f.color ? {
                        background: isActive ? f.color : undefined,
                        borderColor: isActive ? f.color : f.color,
                        color: isActive ? '#fff' : f.color,
                      } : undefined}
                      onClick={() => setStatusFilter(f.value)}
                    />
                  );
                })}
              </HStack>
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
                actions={<Button label="Clear filters" variant="secondary" onClick={() => { setStatusFilter('all'); setSearchQuery(''); }} />}
              />
            ) : (
              <VStack gap={3}>
                {filteredLogs.map((log) => {
                  const isExpanded = expandedId === log.id;
                  const runBadge = runStatusBadge(log.status);
                  return (
                    <Card key={String(log.id)} padding={3} variant="muted">
                      <VStack gap={2}>
                        <HStack justify="between" align="center" wrap="wrap" gap={2}>
                          <VStack gap={1} style={{ minWidth: 0, flex: 1 }}>
                            <HStack gap={2} align="center" wrap="wrap">
                              <Badge variant={runStatusBadgeVariant(log.status)} label={runBadge.label} />
                              <Text type="body" weight="medium">{humanSummary(log)}</Text>
                            </HStack>
                            <Text type="supporting" hasTabularNumbers style={{ fontSize: 'var(--font-size-sm)' }}>
                              {formatTrigger(log.trigger)} {formatDuration(log.started_at, log.completed_at)} <Timestamp value={log.started_at || ''} format="auto" type="supporting" color="secondary" />
                            </Text>
                          </VStack>
                          {hasDetails(log) && (
                            <Button
                              label="Details"
                              variant="ghost"
                              size="sm"
                              icon={<ChevronDown
                                size={12}
                                style={{ transform: isExpanded ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}
                              />}
                              onClick={() => toggleExpand(log.id)}
                            />
                          )}
                        </HStack>

                        {/* Run-level errors shown only when not expanded (expanded view shows them below) */}
                        {log.errors && !isExpanded && (
                          <HStack gap={2} align="start" style={{ color: 'var(--color-error)' }}>
                            <TriangleAlert size={14} />
                            <Text type="supporting">This run completed with errors that need attention.</Text>
                          </HStack>
                        )}

                        {isExpanded && hasDetails(log) && (
                          <VStack gap={2}>
                            {log.items?.length && (
                              <VStack gap={2}>
                                {log.items.map((item, idx) => {
                                  const itemBadge = itemStatusBadge(item.status);
                                  return (
                                    <Card key={idx} padding={2} variant="default">
                                      <HStack justify="between" align="center" wrap="wrap" gap={2}>
                                        <VStack gap={0} style={{ minWidth: 0, flex: 1 }}>
                                          <Text type="body" weight="semibold">{item.sonarr_title}</Text>
                                          <Text type="supporting">{detailLine(item)}</Text>
                                        </VStack>
                                        <Badge variant={itemStatusBadgeVariant(item.status)} label={itemBadge.label} />
                                      </HStack>
                                      {item.error && (
                                        <HStack gap={2} align="start" style={{ color: 'var(--color-error)' }}>
                                          <TriangleAlert size={14} />
                                          <Text type="supporting">{item.error}</Text>
                                        </HStack>
                                      )}
                                    </Card>
                                  );
                                })}
                              </VStack>
                            )}
                            {log.errors && (
                              <Card padding={2} variant="default">
                                <HStack gap={2} align="center">
                                  <TriangleAlert size={14} />
                                  <Text type="body" weight="medium">Run errors</Text>
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
