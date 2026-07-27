import { useEffect, useState, useCallback, useRef } from 'react';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Spinner } from '@astryxdesign/core/Spinner';
import { EmptyState } from '@astryxdesign/core/EmptyState';
import { Icon } from '@astryxdesign/core/Icon';
import { TextInput } from '@astryxdesign/core/TextInput';
import { Search, RefreshCw, History, ChevronDown, TriangleAlert, Check, AlertCircle, XCircle, Loader2 } from 'lucide-react';
import { useAppStore } from '../lib/store';
import { getSyncLogs, triggerSync } from '../lib/api';
import type { SyncLog, SyncLogItem } from '../lib/types';
import { runStatusBadge, itemStatusBadge } from '../lib/fox';

const PAGE_SIZE = 12;

const statusFilters = [
  { value: 'all', label: 'All' },
  { value: 'success', label: 'Success', color: 'var(--color-success)' },
  { value: 'partial', label: 'Partial', color: 'var(--color-warning)' },
  { value: 'failed', label: 'Failed', color: 'var(--color-error)' },
];

function formatTime(dt: string | null | undefined): string {
  if (!dt) return '';
  return new Date(dt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' + new Date(dt).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

function formatTrigger(trigger: string | undefined): string {
  if (!trigger) return '';
  return trigger === 'manual' ? 'Manual' : 'Scheduled';
}

function statusIcon(status: string | undefined) {
  switch (status) {
    case 'success': return <Check size={14} />;
    case 'partial': return <AlertCircle size={14} />;
    case 'failed': return <XCircle size={14} />;
    case 'running': return <Loader2 size={14} className="animate-spin" />;
    default: return <Check size={14} />;
  }
}

function statusColor(status: string | undefined): string {
  switch (status) {
    case 'success': return 'var(--color-success)';
    case 'partial': return 'var(--color-warning)';
    case 'failed': return 'var(--color-error)';
    case 'running': return 'var(--color-accent)';
    default: return 'var(--color-text-secondary)';
  }
}

function hasDetails(log: SyncLog): boolean {
  const hasChanges = (log.items || []).some((item) => (item.episodes_changed || 0) > 0 || item.error);
  return hasChanges || !!log.errors;
}

function detailLine(item: SyncLogItem): string | null {
  const changed = item.episodes_changed || 0;
  return changed > 0 ? `${changed} ${changed === 1 ? 'episode' : 'episodes'} updated` : null;
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
      const itemTitles = (log.items || []).map((i) => (i.sonarr_title || '').toLowerCase()).join(' ');
      if (!itemTitles.includes(q) && !(log.errors || '').toLowerCase().includes(q)) return false;
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
              <div style={{ position: 'relative', paddingLeft: '28px' }}>
                {/* Timeline line */}
                <div style={{
                  position: 'absolute',
                  left: '11px',
                  top: 'var(--spacing-2)',
                  bottom: 'var(--spacing-2)',
                  width: '2px',
                  backgroundColor: 'var(--color-border)',
                }} />

                <VStack gap={4}>
                  {filteredLogs.map((log) => {
                    const isExpanded = expandedId === log.id;
                    const episodes = log.episodes_updated || 0;
                    const anime = log.shows_processed || 0;
                    const canExpand = hasDetails(log);
                    const color = statusColor(log.status);

                    return (
                      <div key={String(log.id)} style={{ position: 'relative' }}>
                        {/* Timeline dot */}
                        <div style={{
                          position: 'absolute',
                          left: '-23px',
                          top: '4px',
                          width: '14px',
                          height: '14px',
                          borderRadius: 'var(--radius-full)',
                          backgroundColor: 'var(--color-background-body)',
                          border: `3px solid ${color}`,
                          zIndex: 1,
                        }} />

                        {/* Content */}
                        <div
                          style={{
                            cursor: canExpand ? 'pointer' : 'default',
                            transition: 'opacity 120ms ease',
                          }}
                          onClick={() => canExpand && toggleExpand(log.id)}
                          onMouseEnter={(e) => { e.currentTarget.style.opacity = '0.8'; }}
                          onMouseLeave={(e) => { e.currentTarget.style.opacity = '1'; }}
                        >
                          <VStack gap={1}>
                            <HStack gap={2} align="center" wrap="wrap">
                              <Text type="body" weight="medium">
                                {episodes > 0 ? `${episodes} ${episodes === 1 ? 'episode' : 'episodes'} updated` : `${anime} anime synced`}
                              </Text>
                              {canExpand && (
                                <ChevronDown
                                  size={14}
                                  style={{
                                    color: 'var(--color-text-secondary)',
                                    transform: isExpanded ? 'rotate(180deg)' : 'none',
                                    transition: 'transform 0.2s',
                                  }}
                                />
                              )}
                            </HStack>
                            <Text type="supporting" color="secondary" hasTabularNumbers style={{ fontSize: 'var(--font-size-sm)' }}>
                              {formatTrigger(log.trigger)} sync {formatTime(log.started_at)}
                            </Text>

                            {log.errors && !isExpanded && (
                              <HStack gap={1} align="center" style={{ color: 'var(--color-error)' }}>
                                <TriangleAlert size={12} />
                                <Text type="supporting" style={{ fontSize: 'var(--font-size-sm)' }}>Completed with errors</Text>
                              </HStack>
                            )}

                            {/* Expanded details */}
                            {isExpanded && canExpand && (
                              <VStack gap={2} style={{ paddingTop: 'var(--spacing-2)' }}>
                                {log.items?.filter((item) => (item.episodes_changed || 0) > 0 || item.error).map((item, i) => {
                                  const itemColor = statusColor(item.status);
                                  return (
                                    <HStack key={i} gap={2} align="start">
                                      <div style={{
                                        flexShrink: 0,
                                        width: 8,
                                        height: 8,
                                        borderRadius: 'var(--radius-full)',
                                        backgroundColor: itemColor,
                                        marginTop: '6px',
                                      }} />
                                      <VStack gap={0} style={{ minWidth: 0, flex: 1 }}>
                                        <Text type="body">{item.sonarr_title}</Text>
                                        {detailLine(item) && (
                                          <Text type="supporting" color="secondary" style={{ fontSize: 'var(--font-size-sm)' }}>{detailLine(item)}</Text>
                                        )}
                                        {item.error && (
                                          <Text type="supporting" style={{ fontSize: 'var(--font-size-sm)', color: 'var(--color-error)' }}>{item.error}</Text>
                                        )}
                                      </VStack>
                                    </HStack>
                                  );
                                })}
                                {log.errors && (
                                  <HStack gap={2} align="start" style={{ color: 'var(--color-error)' }}>
                                    <TriangleAlert size={12} style={{ flexShrink: 0, marginTop: '2px' }} />
                                    <Text type="supporting" style={{ fontSize: 'var(--font-size-sm)' }}>{log.errors}</Text>
                                  </HStack>
                                )}
                              </VStack>
                            )}
                          </VStack>
                        </div>
                      </div>
                    );
                  })}
                </VStack>
              </div>
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
