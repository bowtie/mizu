<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { autoAnimateAction as autoAnimate } from '../../lib/autoAnimate.ts';
  import { Button } from '$lib/components/ui/button/index.ts';
  import { Input } from '$lib/components/ui/input/index.ts';
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
        return 'bg-[rgba(18,184,134,.12)] text-[#63e6be]';
      case 'partial':
        return 'bg-[rgba(253,126,20,.12)] text-[#ffa94d]';
      case 'failed':
        return 'bg-[rgba(224,49,49,.12)] text-[#ffa8a8]';
      case 'running':
        return 'bg-white/[0.06] text-[#adb5bd] animate-pulse';
      default:
        return 'bg-white/[0.06] text-[#adb5bd]';
    }
  }

  function itemStatusTagClass(status: string | undefined): string {
    switch (status) {
      case 'success':
        return 'tag-canon';
      case 'failed':
        return 'tag-filler';
      default:
        return 'tag-anime';
    }
  }

  function statusFilterActiveClass(value: string): string {
    switch (value) {
      case 'success':
        return 'bg-[rgba(18,184,134,.18)] text-[#63e6be]';
      case 'partial':
        return 'bg-[rgba(253,126,20,.18)] text-[#ffa94d]';
      case 'failed':
        return 'bg-[rgba(224,49,49,.18)] text-[#ffa8a8]';
      default:
        return 'bg-accent text-foreground';
    }
  }

  function statusFilterIdleClass(value: string): string {
    switch (value) {
      case 'success':
        return 'text-[#63e6be]/70 hover:text-[#63e6be] hover:bg-[rgba(18,184,134,.1)]';
      case 'partial':
        return 'text-[#ffa94d]/70 hover:text-[#ffa94d] hover:bg-[rgba(253,126,20,.1)]';
      case 'failed':
        return 'text-[#ffa8a8]/70 hover:text-[#ffa8a8] hover:bg-[rgba(224,49,49,.1)]';
      default:
        return 'text-muted-foreground hover:text-foreground hover:bg-accent';
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
      <h1 class="text-2xl font-bold tracking-tight text-foreground">Activity</h1>
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
    <div class="glass-surface flex min-h-56 flex-col items-center justify-center gap-4 rounded-lg text-center">
      <LoaderCircle size={28} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-sm font-semibold text-foreground">Loading activity</h2>
        <p class="text-sm text-muted-foreground">Loading sync history.</p>
      </div>
    </div>
  {:else if logs.length === 0}
    <div class="glass-surface flex min-h-72 flex-col items-center justify-center gap-4 rounded-lg px-6 text-center">
      <span class="flex size-11 items-center justify-center rounded-full border border-rule text-muted-foreground">
        <History size={22} />
      </span>
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">No sync activity yet</h2>
        <p class="max-w-md text-sm leading-6 text-muted-foreground">Run a sync to start building the activity history for your anime library.</p>
      </div>
      <Button variant="default" onclick={handleSync} disabled={syncing}>
        <RefreshCw size={16} />
        Sync now
      </Button>
    </div>
  {:else}
    <!-- Filters bar: segmented mono control + ledger search field -->
    <section class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="inline-flex w-full items-center overflow-hidden rounded-lg border border-rule sm:w-auto" role="group" aria-label="Filter by status">
        {#each statusFilters as filter, i}
          <button
            type="button"
            class={`flex h-10 flex-1 items-center justify-center px-4 font-mono text-xs uppercase tracking-[0.08em] transition-colors duration-150 sm:flex-none ${i > 0 ? 'border-l border-rule' : ''} ${statusFilter === filter.value ? statusFilterActiveClass(filter.value) : statusFilterIdleClass(filter.value)}`}
            onclick={() => (statusFilter = filter.value)}
          >
            {filter.label}
          </button>
        {/each}
      </div>
      <div class="relative sm:w-64">
        <Search size={15} class="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-muted-foreground" />
        <Input bind:value={searchQuery} placeholder="Search logs..." class="rounded-lg pl-9 font-mono text-xs" />
      </div>
    </section>

    <div class="rule-label"><span>Recent runs</span></div>

    <!-- Log list: ledger bands -->
    {#if filteredLogs.length === 0}
      <div class="glass-surface flex min-h-32 flex-col items-center justify-center gap-2 rounded-lg text-center">
        <p class="text-sm font-medium text-foreground">No matching logs</p>
        <p class="text-sm text-muted-foreground">Try adjusting your filters or search query.</p>
      </div>
    {:else}
      <section use:autoAnimate>
        {#each filteredLogs as log (log.id)}
          {@const runBadge = runStatusBadge(log.status)}
          <div class="border-b border-[#10141b] py-3.5">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0 space-y-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class={`rounded-[5px] px-1.5 py-[2px] font-mono text-[10px] uppercase tracking-[0.1em] ${runStatusChipClass(log.status)}`}>{runBadge.label}</span>
                  <h2 class="text-[13px] font-medium text-foreground">{humanSummary(log)}</h2>
                </div>
                <p class="font-mono text-[10px] text-[#868e96]">
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
                  class="flex shrink-0 items-center gap-1.5 font-mono text-[10px] uppercase tracking-[0.08em] text-primary transition-colors hover:text-primary/80"
                  onclick={() => toggleExpand(log.id)}
                  aria-expanded={expandedId === log.id}
                >
                  Details
                  <ChevronDown size={12} class={`transition-transform duration-200 ${expandedId === log.id ? 'rotate-180' : ''}`} />
                </button>
              {/if}
            </div>

            {#if log.errors}
              <div class="mt-2 rounded-lg border border-[rgba(224,49,49,.25)] bg-[rgba(224,49,49,.08)] px-3 py-2 text-[13px] leading-5 text-[#ffa8a8]">
                This run completed with errors that need attention.
              </div>
            {/if}

            {#if expandedId === log.id && hasDetails(log)}
              <div class="mt-3 space-y-2" transition:slide={{ duration: 220 }}>
                {#if log.items?.length}
                  <div class="space-y-2">
                    {#each log.items as item}
                      {@const itemBadge = itemStatusBadge(item.status)}
                      <div class="glass-inset rounded-lg p-3">
                        <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                          <div class="space-y-0.5">
                            <h3 class="text-sm font-semibold text-foreground">{item.sonarr_title}</h3>
                            <p class="text-[13px] leading-5 text-muted-foreground">{detailLine(item)}</p>
                          </div>
                          <span class={`tag ${itemStatusTagClass(item.status)} shrink-0`}>{itemBadge.label}</span>
                        </div>

                        {#if item.error}
                          <div class="mt-2 flex items-start gap-2 rounded-lg border border-[rgba(224,49,49,.25)] bg-[rgba(224,49,49,.08)] px-3 py-2 text-[13px] leading-5 text-[#ffa8a8]">
                            <TriangleAlert size={14} class="mt-0.5 shrink-0" />
                            <span>{item.error}</span>
                          </div>
                        {/if}
                      </div>
                    {/each}
                  </div>
                {/if}

                {#if log.errors}
                  <div class="rounded-lg border border-[rgba(224,49,49,.25)] bg-[rgba(224,49,49,.08)] p-3 text-[13px] leading-5 text-[#ffa8a8]">
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
          class="inline-flex items-center gap-2 rounded-lg border border-rule bg-card px-4 py-2 font-mono text-[11px] uppercase tracking-[0.08em] text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
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
