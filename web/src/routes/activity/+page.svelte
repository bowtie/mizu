<script>
  import { onDestroy, onMount } from 'svelte';
  import { autoAnimate } from '@formkit/auto-animate';
  import { slide } from 'svelte/transition';
  import { Badge, Box, Button, Input } from '@foxui/core';
  import { addNotification } from '../../lib/stores.js';
  import { getSyncLogs, triggerSync } from '../../lib/api.js';
  import { itemStatusBadge, runStatusBadge } from '../../lib/fox.js';
  import { ChevronDown, History, LoaderCircle, RefreshCw, Search, TriangleAlert } from 'lucide-svelte';

  const PAGE_SIZE = 8;

  let loading = true;
  let logs = [];
  let syncing = false;
  let loadingMore = false;
  let hasMore = false;
  let nextOffset = 0;
  let expandedId = null;
  let statusFilter = 'all';
  let searchQuery = '';
  let pollTimer = null;

  const statusFilters = [
    { value: 'all', label: 'All' },
    { value: 'success', label: 'Success' },
    { value: 'partial', label: 'Partial' },
    { value: 'failed', label: 'Failed' },
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
      logs = reset ? (page.items || []) : [...logs, ...(page.items || [])];
      hasMore = !!page.has_more;
      nextOffset = page.next_offset || logs.length;
    } catch (err) {
      addNotification('Failed to load activity: ' + err.message, 'error');
    }

    loading = false;
    loadingMore = false;
  }

  async function handleSync() {
    syncing = true;
    try {
      await triggerSync();
      addNotification('Sync triggered successfully', 'success');
      // Poll for updates while sync is running
      startPolling();
    } catch (err) {
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
      const hasRunning = logs.some((l) => l.status === 'running');
      if (hasRunning && attempts < 20) {
        pollTimer = setTimeout(poll, 2000);
      }
    };
    pollTimer = setTimeout(poll, 1800);
  }

  function toggleExpand(id) {
    expandedId = expandedId === id ? null : id;
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return 'Running';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }

  function formatTrigger(trigger) {
    if (!trigger) return '-';
    return trigger === 'manual' ? 'Manual' : 'Scheduled';
  }

  function humanSummary(log) {
    const anime = log.shows_processed || 0;
    const episodes = log.episodes_updated || 0;
    return `Checked ${anime} anime, changed ${episodes} ${episodes === 1 ? 'episode' : 'episodes'}.`;
  }

  function detailLine(item) {
    if (item.summary) return item.summary;
    return item.episodes_changed > 0
      ? `${item.episodes_changed} ${item.episodes_changed === 1 ? 'episode change' : 'episode changes'}.`
      : 'No episode changes.';
  }

  function hasDetails(log) {
    return !!(log.items?.length || log.errors);
  }

  $: filteredLogs = logs.filter((log) => {
    if (statusFilter !== 'all' && log.status !== statusFilter) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      const summary = humanSummary(log).toLowerCase();
      const itemTitles = (log.items || []).map((i) => (i.sonarr_title || '').toLowerCase()).join(' ');
      if (!summary.includes(q) && !itemTitles.includes(q) && !(log.errors || '').toLowerCase().includes(q)) return false;
    }
    return true;
  });
</script>

<svelte:head>
  <title>Mizu - Activity</title>
</svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-base-50 sm:text-3xl">Activity</h1>
      <p class="mt-1 text-sm text-base-400">Sync history and per-anime change logs.</p>
    </div>
    <Button variant="primary" onclick={handleSync} disabled={syncing}>
      {#if syncing}
        <LoaderCircle size={16} class="animate-spin" />
      {:else}
        <RefreshCw size={16} />
      {/if}
      {syncing ? 'Syncing...' : 'Sync now'}
    </Button>
  </section>

  {#if loading}
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-2xl border border-base-800/80 bg-base-950/60 text-center">
      <LoaderCircle size={30} class="animate-spin text-accent-400" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-base-100">Loading activity</h2>
        <p class="text-sm text-base-400">Surfacing recent sync history.</p>
      </div>
    </Box>
  {:else if logs.length === 0}
    <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-2xl border border-base-800/80 bg-base-950/60 px-6 text-center">
      <span class="flex size-12 items-center justify-center rounded-xl border border-accent-500/20 bg-accent-950/30 text-accent-300">
        <History size={24} />
      </span>
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-base-50">No sync activity yet</h2>
        <p class="max-w-md text-sm leading-6 text-base-400">Run a sync to start building the activity history for your anime library.</p>
      </div>
      <Button variant="primary" onclick={handleSync} disabled={syncing}>
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
            class="rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors {statusFilter === filter.value
              ? 'border-accent-500/40 bg-accent-950/20 text-accent-200'
              : 'border-base-800/80 bg-base-950/60 text-base-400 hover:text-base-200'}"
            onclick={() => (statusFilter = filter.value)}
          >
            {filter.label}
          </button>
        {/each}
      </div>
      <div class="relative sm:w-64">
        <Search size={16} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-base-500" />
        <Input bind:value={searchQuery} placeholder="Search logs..." class="pl-9" />
      </div>
    </section>

    <!-- Log list -->
    {#if filteredLogs.length === 0}
      <Box class="flex min-h-32 flex-col items-center justify-center gap-2 rounded-2xl border border-base-800/80 bg-base-950/60 text-center">
        <p class="text-sm font-medium text-base-200">No matching logs</p>
        <p class="text-sm text-base-400">Try adjusting your filters or search query.</p>
      </Box>
    {:else}
      <section class="space-y-4" use:autoAnimate>
        {#each filteredLogs as log (log.id)}
          {@const runBadge = runStatusBadge(log.status)}
          <Box class="rounded-2xl border border-base-800/80 bg-base-950/60">
            <div class="space-y-4">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div class="space-y-3">
                  <div class="flex flex-wrap gap-2">
                    <Badge variant={runBadge.variant} size="sm">{runBadge.label}</Badge>
                    <Badge variant="secondary" size="sm">{formatTrigger(log.trigger)}</Badge>
                    <Badge variant="secondary" size="sm">{formatDuration(log.started_at, log.completed_at)}</Badge>
                  </div>

                  <div class="space-y-1">
                    <h2 class="text-base font-semibold text-base-50">{humanSummary(log)}</h2>
                    <p class="text-sm text-base-400">{formatTime(log.started_at)}</p>
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
                <div class="rounded-xl border border-rose-500/20 bg-rose-950/20 px-4 py-3 text-sm text-rose-200">
                  This run completed with errors that need attention.
                </div>
              {/if}

              {#if expandedId === log.id && hasDetails(log)}
                <div class="space-y-4 pt-1" transition:slide={{ duration: 220 }}>
                  {#if log.items?.length}
                    <div class="space-y-3">
                      {#each log.items as item}
                        {@const itemBadge = itemStatusBadge(item.status)}
                        <div class="rounded-xl border border-base-800/80 bg-base-950/70 p-4">
                          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                            <div class="space-y-2">
                              <h3 class="text-sm font-semibold text-base-50">{item.sonarr_title}</h3>
                              <p class="text-sm leading-6 text-base-400">{detailLine(item)}</p>
                            </div>
                            <Badge variant={itemBadge.variant} size="sm">{itemBadge.label}</Badge>
                          </div>

                          {#if item.error}
                            <div class="mt-3 flex items-start gap-2 rounded-xl border border-rose-500/20 bg-rose-950/20 px-3 py-2 text-sm text-rose-200">
                              <TriangleAlert size={16} class="mt-0.5 shrink-0" />
                              <span>{item.error}</span>
                            </div>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {/if}

                  {#if log.errors}
                    <div class="rounded-xl border border-rose-500/20 bg-rose-950/20 p-4 text-sm text-rose-200">
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
