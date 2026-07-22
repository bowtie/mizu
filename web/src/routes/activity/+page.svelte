<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { autoAnimateAction as autoAnimate } from '../../lib/autoAnimate.ts';
  import { addNotification } from '../../lib/stores.ts';
  import { getSyncLogs, triggerSync } from '../../lib/api.ts';
  import type { SyncLog, SyncLogItem } from '$lib/types.ts';
  import { itemStatusBadge, runStatusBadge } from '../../lib/fox.ts';
  import { ChevronDown, History, LoaderCircle, RefreshCw, Search, TriangleAlert } from 'lucide-svelte';

  const PAGE_SIZE = 8;

  let loading = $state(true);
  let logs = $state<SyncLog[]>([]);
  let syncing = $state(false);
  let loadingMore = $state(false);
  let hasMore = $state(false);
  let nextOffset = $state(0);
  let expandedId = $state<string | number | null>(null);
  let statusFilter = $state('all');
  let searchQuery = $state('');
  let pollTimer: ReturnType<typeof setTimeout> | null = null;

  interface StatusFilter {
    value: string;
    label: string;
  }

  const statusFilters: StatusFilter[] = [
    { value: 'all', label: 'All' },
    { value: 'success', label: 'Success' },
    { value: 'partial', label: 'Partial' },
    { value: 'failed', label: 'Failed' },
  ];

  let filteredLogs = $derived(
    logs.filter((log: SyncLog) => {
      if (statusFilter !== 'all' && log.status !== statusFilter) return false;
      if (searchQuery) {
        const q = searchQuery.toLowerCase();
        const summary = humanSummary(log).toLowerCase();
        const itemTitles = (log.items || []).map((i: SyncLogItem) => (i.sonarr_title || '').toLowerCase()).join(' ');
        if (!summary.includes(q) && !itemTitles.includes(q) && !(log.errors || '').toLowerCase().includes(q)) return false;
      }
      return true;
    })
  );

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

  function runStatusChipClass(status: string | undefined): string {
    switch (status) {
      case 'success':
        return 'preset-filled-success-500 text-surface-50';
      case 'partial':
        return 'preset-filled-warning-500 text-surface-50';
      case 'failed':
        return 'preset-filled-error-500 text-surface-50';
      case 'running':
        return 'preset-tonal-surface text-surface-200 animate-pulse';
      default:
        return 'preset-tonal-surface text-surface-200';
    }
  }

  function itemStatusBadgeClass(status: string | undefined): string {
    switch (status) {
      case 'success':
        return 'preset-tonal-success';
      case 'failed':
        return 'preset-tonal-error';
      default:
        return 'preset-tonal-secondary';
    }
  }

  function statusFilterActiveClass(value: string): string {
    switch (value) {
      case 'success':
        return 'preset-filled-success-500 text-surface-50';
      case 'partial':
        return 'preset-filled-warning-500 text-surface-50';
      case 'failed':
        return 'preset-filled-error-500 text-surface-50';
      default:
        return 'preset-filled-surface-500 text-surface-50';
    }
  }

  function statusFilterIdleClass(value: string): string {
    switch (value) {
      case 'success':
        return 'text-success-500/70 hover:text-success-500 hover:preset-tonal-success-500';
      case 'partial':
        return 'text-warning-500/70 hover:text-warning-500 hover:preset-tonal-warning-500';
      case 'failed':
        return 'text-error-500/70 hover:text-error-500 hover:preset-tonal-error-500';
      default:
        return 'text-surface-400 hover:text-surface-50 hover:bg-surface-800';
    }
  }
</script>

<svelte:head>
  <title>Mizu - Activity</title>
</svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-surface-50">Activity</h1>
      <p class="mt-1 text-sm text-surface-400">Sync history and per-anime change logs.</p>
    </div>
    <button class="btn btn-base preset-filled-primary-500" onclick={handleSync} disabled={syncing}>
      {#if syncing}
        <LoaderCircle size={16} class="animate-spin" />
      {:else}
        <RefreshCw size={16} />
      {/if}
      {syncing ? 'Syncing...' : 'Sync now'}
    </button>
  </section>

  {#if loading}
    <div class="card preset-tonal-surface flex min-h-56 flex-col items-center justify-center gap-4 text-center">
      <LoaderCircle size={28} class="animate-spin text-primary-500" />
      <div class="space-y-1">
        <h2 class="text-sm font-semibold text-surface-50">Loading activity</h2>
        <p class="text-sm text-surface-400">Loading sync history.</p>
      </div>
    </div>
  {:else if logs.length === 0}
    <div class="card preset-tonal-surface flex min-h-72 flex-col items-center justify-center gap-4 px-6 text-center">
      <span class="flex size-11 items-center justify-center rounded-full border border-surface-700 text-surface-400">
        <History size={22} />
      </span>
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-surface-50">No sync activity yet</h2>
        <p class="max-w-md text-sm leading-6 text-surface-400">Run a sync to start building the activity history for your anime library.</p>
      </div>
      <button class="btn btn-base preset-filled-primary-500" onclick={handleSync} disabled={syncing}>
        <RefreshCw size={16} />
        Sync now
      </button>
    </div>
  {:else}
    <!-- Filters bar: segmented mono control + search field -->
    <section class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="inline-flex w-full items-center overflow-hidden rounded-lg border border-surface-800 sm:w-auto" role="group" aria-label="Filter by status">
        {#each statusFilters as filter, i}
          <button
            type="button"
            class={`flex h-10 flex-1 items-center justify-center px-4 text-xs uppercase tracking-[0.08em] transition-colors duration-150 sm:flex-none ${i > 0 ? 'border-l border-surface-800' : ''} ${statusFilter === filter.value ? statusFilterActiveClass(filter.value) : statusFilterIdleClass(filter.value)}`}
            onclick={() => (statusFilter = filter.value)}
          >
            {filter.label}
          </button>
        {/each}
      </div>
      <div class="relative sm:w-64">
        <Search size={15} class="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-surface-400" />
        <input bind:value={searchQuery} placeholder="Search logs..." class="input field-base rounded-lg pl-9 text-xs" />
      </div>
    </section>

    <div class="rule-label"><span>Recent runs</span></div>

    <!-- Log list -->
    {#if filteredLogs.length === 0}
      <div class="card preset-tonal-surface flex min-h-32 flex-col items-center justify-center gap-2 text-center">
        <p class="text-sm font-medium text-surface-50">No matching logs</p>
        <p class="text-sm text-surface-400">Try adjusting your filters or search query.</p>
      </div>
    {:else}
      <section use:autoAnimate>
        {#each filteredLogs as log (log.id)}
          {@const runBadge = runStatusBadge(log.status)}
          <div class="border-b border-surface-800 py-3.5">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0 space-y-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class={`badge ${runStatusChipClass(log.status)} badge-sm text-[10px] uppercase tracking-[0.1em]`}>{runBadge.label}</span>
                  <h2 class="text-[13px] font-medium text-surface-50">{humanSummary(log)}</h2>
                </div>
                <p class="text-[10px] text-surface-500">
                  {formatTrigger(log.trigger)}
                  <span aria-hidden="true"> &middot; </span>
                  <span class="tabular-nums">{formatDuration(log.started_at, log.completed_at)}</span>
                  <span aria-hidden="true"> &middot; </span>
                  {formatTime(log.started_at)}
                </p>
              </div>

              {#if hasDetails(log)}
                <button
                  type="button"
                  class="flex shrink-0 items-center gap-1.5 text-[10px] uppercase tracking-[0.08em] text-primary-500 transition-colors hover:text-primary-400"
                  onclick={() => toggleExpand(log.id)}
                  aria-expanded={expandedId === log.id}
                >
                  Details
                  <ChevronDown size={12} class={`transition-transform duration-200 ${expandedId === log.id ? 'rotate-180' : ''}`} />
                </button>
              {/if}
            </div>

            {#if log.errors}
              <div class="mt-2 card preset-filled-error-500 px-3 py-2 text-[13px] leading-5">
                This run completed with errors that need attention.
              </div>
            {/if}

            {#if expandedId === log.id && hasDetails(log)}
              <div class="mt-3 space-y-2" transition:slide={{ duration: 220 }}>
                {#if log.items?.length}
                  <div class="space-y-2">
                    {#each log.items as item}
                      {@const itemBadge = itemStatusBadge(item.status)}
                      <div class="rounded-lg border border-surface-800 bg-surface-950 p-3">
                        <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                          <div class="space-y-0.5">
                            <h3 class="text-sm font-semibold text-surface-50">{item.sonarr_title}</h3>
                            <p class="text-[13px] leading-5 text-surface-400">{detailLine(item)}</p>
                          </div>
                          <span class="badge badge-sm text-[10px] uppercase tracking-[0.08em] {itemStatusBadgeClass(item.status)} shrink-0">{itemBadge.label}</span>
                        </div>

                        {#if item.error}
                          <div class="mt-2 flex items-start gap-2 card preset-filled-error-500 px-3 py-2 text-[13px] leading-5">
                            <TriangleAlert size={14} class="mt-0.5 shrink-0" />
                            <span>{item.error}</span>
                          </div>
                        {/if}
                      </div>
                    {/each}
                  </div>
                {/if}

                {#if log.errors}
                  <div class="card preset-filled-error-500 p-3 text-[13px] leading-5">
                    <div class="mb-1.5 flex items-center gap-2 font-medium">
                      <TriangleAlert size={14} />
                      Run errors
                    </div>
                    {log.errors}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </section>
    {/if}

    {#if hasMore}
      <div class="flex justify-center">
        <button
          type="button"
          class="btn btn-base preset-tonal"
          onclick={() => loadActivity(false)}
          disabled={loadingMore}
        >
          {#if loadingMore}
            <LoaderCircle size={14} class="animate-spin" />
          {/if}
          {loadingMore ? 'Loading...' : 'Load more'}
        </button>
      </div>
    {/if}
  {/if}
</div>
