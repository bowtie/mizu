<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { autoAnimateAction as autoAnimate } from '../../lib/autoAnimate.ts';
  import { Badge } from '$lib/components/ui/badge/index.ts';
  import { Button } from '$lib/components/ui/button/index.ts';
  import { Input } from '$lib/components/ui/input/index.ts';
  import Box from '$lib/components/Box.svelte';
  import { addNotification } from '../../lib/stores.ts';
  import { getSyncLogs, triggerSync } from '../../lib/api.ts';
  import type { SyncLog, SyncLogItem } from '$lib/types.ts';
  import { itemStatusBadge, runStatusBadge } from '../../lib/fox.ts';
  import { ChevronDown, History, LoaderCircle, RefreshCw, Search, TriangleAlert } from 'lucide-svelte';

  const PAGE_SIZE = 8;

  let loading = true;
  let logs: SyncLog[] = [];
  let syncing = false;
  let loadingMore = false;
  let hasMore = false;
  let nextOffset = 0;
  let expandedId: string | number | null = null;
  let statusFilter = 'all';
  let searchQuery = '';
  let pollTimer: ReturnType<typeof setTimeout> | null = null;

  interface StatusFilter {
    value: string;
    label: string;
    activeClass: string;
    hoverClass: string;
  }

  const statusFilters: StatusFilter[] = [
    { value: 'all', label: 'All', activeClass: 'border-primary/40 bg-primary/20 text-primary', hoverClass: 'hover:border-primary/60 hover:bg-primary/30 hover:text-primary' },
    { value: 'success', label: 'Success', activeClass: 'border-success/40 bg-success/20 text-success', hoverClass: 'hover:border-success/60 hover:bg-success/30 hover:text-success' },
    { value: 'partial', label: 'Partial', activeClass: 'border-warning/40 bg-warning/20 text-warning', hoverClass: 'hover:border-warning/60 hover:bg-warning/30 hover:text-warning' },
    { value: 'failed', label: 'Failed', activeClass: 'border-destructive/40 bg-destructive/20 text-destructive', hoverClass: 'hover:border-destructive/60 hover:bg-destructive/30 hover:text-destructive' },
  ];

  onMount(async () => {
    await loadActivity(true);
  });

  onDestroy(() => {
    if (pollTimer) clearTimeout(pollTimer);
  });

  async function loadActivity(reset = true) {
    if (reset) {
      loading = logs.length === 0;
    } else {
      loadingMore = true;
    }

    try {
      const page = (await getSyncLogs(PAGE_SIZE, reset ? 0 : nextOffset)) || { items: [], has_more: false, next_offset: 0 };
      const newLogs: SyncLog[] = page.items || [];
      logs = reset ? newLogs : [...logs, ...newLogs];
      hasMore = !!page.has_more;
      nextOffset = page.next_offset ?? logs.length;
    } catch (err: any) {
      addNotification('Failed to load activity: ' + err.message, 'error');
    }

    loading = false;
    loadingMore = false;
  }

  async function handleSync() {
    syncing = true;
    try {
      await triggerSync();
      addNotification('Sync started', 'info');
      // Poll for updates while sync is running
      startPolling();
    } catch (err: any) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    syncing = false;
  }

  function startPolling() {
    if (pollTimer) clearTimeout(pollTimer);
    let attempts = 0;
    const poll = async () => {
      attempts++;
      await loadActivity(true);
      const hasRunning = logs.some((l: SyncLog) => l.status === 'running');
      if (hasRunning && attempts < 20) {
        pollTimer = setTimeout(poll, 2000);
      }
    };
    pollTimer = setTimeout(poll, 1800);
  }

  function toggleExpand(id: string | number) {
    expandedId = expandedId === id ? null : id;
  }

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

  $: filteredLogs = logs.filter((log: SyncLog) => {
    if (statusFilter !== 'all' && log.status !== statusFilter) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      const summary = humanSummary(log).toLowerCase();
      const itemTitles = (log.items || []).map((i: SyncLogItem) => (i.sonarr_title || '').toLowerCase()).join(' ');
      if (!summary.includes(q) && !itemTitles.includes(q) && !(log.errors || '').toLowerCase().includes(q)) return false;
    }
    return true;
  });
</script>

<svelte:head>
  <title>Mizu - Activity</title>
</svelte:head>

<div class="space-y-8">
  <!-- Header -->
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">Activity</h1>
      <p class="mt-1 text-sm text-muted-foreground">Sync history and per-anime change logs.</p>
    </div>
    <Button variant="default" onclick={handleSync} disabled={syncing}>
      {#if syncing}
        <LoaderCircle size={16} class="animate-spin" />
      {:else}
        <RefreshCw size={16} />
      {/if}
      {syncing ? 'Syncing...' : 'Sync now'}
    </Button>
  </section>

  {#if loading}
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-xl glass-card text-center">
      <LoaderCircle size={30} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">Loading activity</h2>
        <p class="text-sm text-muted-foreground">Loading sync history.</p>
      </div>
    </Box>
  {:else if logs.length === 0}
    <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-xl glass-card px-6 text-center">
      <span class="flex size-12 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
        <History size={24} />
      </span>
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-foreground">No sync activity yet</h2>
        <p class="max-w-md text-sm leading-6 text-muted-foreground">Run a sync to start building the activity history for your anime library.</p>
      </div>
      <Button variant="default" onclick={handleSync} disabled={syncing}>
        <RefreshCw size={16} />
        Sync now
      </Button>
    </Box>
  {:else}
    <!-- Filters bar -->
    <section class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex flex-wrap gap-1.5">
        {#each statusFilters as filter}
          <button
            type="button"
            class={`rounded-xl border px-3 py-1.5 text-sm font-medium backdrop-blur-md saturate-150 transition-colors duration-200 ${statusFilter === filter.value ? `${filter.activeClass} ${filter.hoverClass}` : `border-white/[0.08] bg-white/[0.03] text-muted-foreground ${filter.hoverClass}`}`}
            onclick={() => (statusFilter = filter.value)}
          >
            {filter.label}
          </button>
        {/each}
      </div>
      <div class="relative sm:w-64">
        <Search size={16} class="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-muted-foreground" />
        <Input bind:value={searchQuery} placeholder="Search logs..." class="pl-9" />
      </div>
    </section>

    <!-- Log list -->
    {#if filteredLogs.length === 0}
      <Box class="flex min-h-32 flex-col items-center justify-center gap-2 rounded-xl glass-card text-center">
        <p class="text-sm font-medium text-foreground">No matching logs</p>
        <p class="text-sm text-muted-foreground">Try adjusting your filters or search query.</p>
      </Box>
    {:else}
      <section class="space-y-4" use:autoAnimate>
        {#each filteredLogs as log (log.id)}
          {@const runBadge = runStatusBadge(log.status)}
          <Box class="rounded-xl glass-card p-5">
            <div class="space-y-4">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div class="space-y-3">
                  <div class="flex flex-wrap gap-2">
                    <Badge variant={runBadge.variant}>{runBadge.label}</Badge>
                    <Badge variant="secondary">{formatTrigger(log.trigger)}</Badge>
                    <Badge variant="secondary">{formatDuration(log.started_at, log.completed_at)}</Badge>
                  </div>

                  <div class="space-y-1">
                    <h2 class="text-base font-semibold text-foreground">{humanSummary(log)}</h2>
                    <p class="text-sm text-muted-foreground">{formatTime(log.started_at)}</p>
                  </div>
                </div>

                {#if hasDetails(log)}
                  <Button variant="ghost" onclick={() => toggleExpand(log.id)} data-current={expandedId === log.id ? 'true' : undefined}>
                    Details
                    <ChevronDown size={16} class={expandedId === log.id ? 'rotate-180' : ''} />
                  </Button>
                {/if}
              </div>

              {#if log.errors}
                <div class="rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  This run completed with errors that need attention.
                </div>
              {/if}

              {#if expandedId === log.id && hasDetails(log)}
                <div class="space-y-4 pt-1" transition:slide={{ duration: 220 }}>
                  {#if log.items?.length}
                    <div class="space-y-3">
                      {#each log.items as item}
                        {@const itemBadge = itemStatusBadge(item.status)}
                        <div class="rounded-xl glass-card p-4">
                          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                            <div class="space-y-2">
                              <h3 class="text-sm font-semibold text-foreground">{item.sonarr_title}</h3>
                              <p class="text-sm leading-6 text-muted-foreground">{detailLine(item)}</p>
                            </div>
                            <Badge variant={itemBadge.variant}>{itemBadge.label}</Badge>
                          </div>

                          {#if item.error}
                            <div class="mt-3 flex items-start gap-2 rounded-xl border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                              <TriangleAlert size={16} class="mt-0.5 shrink-0" />
                              <span>{item.error}</span>
                            </div>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {/if}

                  {#if log.errors}
                    <div class="rounded-xl border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
                      <div class="mb-2 flex items-center gap-2 font-medium">
                        <TriangleAlert size={16} />
                        Run errors
                      </div>
                      {log.errors}
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          </Box>
        {/each}
      </section>
    {/if}

    {#if hasMore}
      <div class="flex justify-center">
        <Button variant="secondary" onclick={() => loadActivity(false)} disabled={loadingMore}>
          {#if loadingMore}
            <LoaderCircle size={16} class="animate-spin" />
          {/if}
          {loadingMore ? 'Loading...' : 'Load more'}
        </Button>
      </div>
    {/if}
  {/if}
</div>
