<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Badge, Box, Button } from '@foxui/core';
  import { shows, syncStatus, settings, addNotification } from '../lib/stores.js';
  import { getShows, getStats, getSyncLogs, triggerSync, getSettings, getStatus } from '../lib/api.js';
  import { runStatusBadge, syncStateBadge } from '../lib/fox.js';
  import {
    ArrowRight,
    CircleAlert,
    CircleCheckBig,
    History,
    LoaderCircle,
    Plus,
    RefreshCw,
    TriangleAlert,
    Tv,
    Waves,
  } from 'lucide-svelte';

  let loading = true;
  let syncing = false;
  let stats = null;
  let recentLogs = [];
  let connected = false;

  onMount(async () => {
    await Promise.all([loadDashboard(), loadShows(), loadSettings(), loadStatus()]);
  });

  async function loadDashboard() {
    try {
      const [statsRes, logsRes] = await Promise.all([
        getStats(),
        getSyncLogs(3, 0),
      ]);
      stats = statsRes;
      recentLogs = logsRes?.items || [];
    } catch (err) {
      addNotification('Failed to load dashboard: ' + err.message, 'error');
    }
    loading = false;
  }

  async function loadShows() {
    try {
      const data = await getShows();
      shows.set(data || []);
    } catch {}
  }

  async function loadSettings() {
    try {
      const data = await getSettings();
      settings.set(data);
    } catch {}
  }

  async function loadStatus() {
    try {
      const data = await getStatus();
      connected = data?.status === 'ok';
      syncStatus.update((s) => ({ ...s, connected }));
    } catch {
      connected = false;
    }
  }

  async function handleSync() {
    syncing = true;
    try {
      await triggerSync();
      addNotification('Sync triggered successfully', 'success');
      setTimeout(() => loadDashboard(), 1800);
    } catch (err) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    syncing = false;
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return 'Running';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
  }

  function humanSummary(log) {
    const anime = log.shows_processed || 0;
    const episodes = log.episodes_updated || 0;
    return `Checked ${anime} anime, changed ${episodes} ${episodes === 1 ? 'episode' : 'episodes'}.`;
  }

  $: showCount = $shows.length;
  $: pausedCount = ($shows || []).filter((s) => !s.enabled).length;
  $: lastSync = recentLogs[0] || null;
  $: failedSyncs = recentLogs.filter((l) => l.status === 'failed' || l.status === 'partial');
  $: needsAttention = failedSyncs.length > 0 || pausedCount > 0;

  const statCards = [
    { label: 'Anime tracked', key: 'shows', icon: Tv, accent: 'text-accent-300' },
    { label: 'Total episodes', key: 'total', icon: Waves, accent: 'text-blue-300' },
    { label: 'Filler managed', key: 'filler', icon: CircleCheckBig, accent: 'text-rose-300' },
    { label: 'Canon episodes', key: 'canon', icon: CircleCheckBig, accent: 'text-emerald-300' },
  ];
</script>

<svelte:head>
  <title>Mizu - Dashboard</title>
</svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-base-50 sm:text-3xl">Dashboard</h1>
      <p class="mt-1 text-sm text-base-400">
        {#if showCount > 0}
          Tracking {showCount} {showCount === 1 ? 'anime' : 'anime'} · {stats?.total || 0} episodes
        {:else}
          No anime tracked yet
        {/if}
      </p>
    </div>
    <div class="flex gap-2">
      <Button variant="secondary" onclick={() => goto('/shows')}>
        <Tv size={16} />
        Browse anime
      </Button>
      <Button variant="primary" onclick={handleSync} disabled={syncing}>
        {#if syncing}
          <LoaderCircle size={16} class="animate-spin" />
        {:else}
          <RefreshCw size={16} />
        {/if}
        {syncing ? 'Syncing...' : 'Sync now'}
      </Button>
    </div>
  </section>

  {#if loading}
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-2xl border border-base-800/80 bg-base-950/60 text-center">
      <LoaderCircle size={30} class="animate-spin text-accent-400" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-base-100">Loading dashboard</h2>
        <p class="text-sm text-base-400">Gathering your library overview.</p>
      </div>
    </Box>
  {:else}
    <!-- Needs attention -->
    {#if needsAttention}
      <section class="space-y-3">
        <h2 class="flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-base-400">
          <CircleAlert size={16} class="text-amber-400" />
          Needs attention
        </h2>
        <div class="grid gap-3 sm:grid-cols-2">
          {#if failedSyncs.length > 0}
            <Box class="rounded-2xl border border-rose-500/20 bg-rose-950/20 p-4">
              <div class="flex items-start gap-3">
                <span class="flex size-9 shrink-0 items-center justify-center rounded-lg border border-rose-500/30 bg-rose-950/40 text-rose-300">
                  <TriangleAlert size={18} />
                </span>
                <div class="space-y-1">
                  <h3 class="text-sm font-semibold text-base-50">
                    {failedSyncs.length} {failedSyncs.length === 1 ? 'sync run needs review' : 'sync runs need review'}
                  </h3>
                  <p class="text-sm text-base-400">Recent syncs completed with errors or partial results.</p>
                  <Button variant="ghost" size="sm" onclick={() => goto('/activity')} class="mt-2 -ml-2">
                    View activity
                    <ArrowRight size={14} />
                  </Button>
                </div>
              </div>
            </Box>
          {/if}
          {#if pausedCount > 0}
            <Box class="rounded-2xl border border-base-800/80 bg-base-950/60 p-4">
              <div class="flex items-start gap-3">
                <span class="flex size-9 shrink-0 items-center justify-center rounded-lg border border-base-700 bg-base-900 text-base-300">
                  <Waves size={18} />
                </span>
                <div class="space-y-1">
                  <h3 class="text-sm font-semibold text-base-50">
                    {pausedCount} {pausedCount === 1 ? 'anime paused' : 'anime paused'}
                  </h3>
                  <p class="text-sm text-base-400">Paused anime are not being synced automatically.</p>
                  <Button variant="ghost" size="sm" onclick={() => goto('/shows')} class="mt-2 -ml-2">
                    Manage anime
                    <ArrowRight size={14} />
                  </Button>
                </div>
              </div>
            </Box>
          {/if}
        </div>
      </section>
    {/if}

    <!-- Stats grid -->
    {#if stats}
      <section class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {#each statCards as card}
          <Box class="rounded-2xl border border-base-800/80 bg-base-950/60 p-4">
            <div class="flex items-center gap-3">
              <span class="flex size-9 shrink-0 items-center justify-center rounded-lg border border-base-800 bg-base-900 {card.accent}">
                <card.icon size={18} />
              </span>
              <div>
                <p class="text-xl font-bold tabular-nums text-base-50">
                  {(card.key === 'shows' ? showCount : stats[card.key] || 0).toLocaleString()}
                </p>
                <p class="text-xs text-base-400">{card.label}</p>
              </div>
            </div>
          </Box>
        {/each}
      </section>
    {/if}

    <!-- Last sync + quick actions -->
    <div class="grid gap-4 lg:grid-cols-3">
      <!-- Last sync summary -->
      <section class="lg:col-span-2">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-base-400">Last sync</h2>
        {#if lastSync}
          {@const lastBadge = runStatusBadge(lastSync.status)}
          <Box class="rounded-2xl border border-base-800/80 bg-base-950/60 p-5">
            <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div class="space-y-2">
                <div class="flex flex-wrap gap-2">
                  <Badge variant={lastBadge.variant} size="sm">{lastBadge.label}</Badge>
                  <Badge variant="secondary" size="sm">{formatDuration(lastSync.started_at, lastSync.completed_at)}</Badge>
                </div>
                <h3 class="text-base font-semibold text-base-50">{humanSummary(lastSync)}</h3>
                <p class="text-sm text-base-400">{formatTime(lastSync.started_at)}</p>
              </div>
              <Button variant="ghost" size="sm" onclick={() => goto('/activity')}>
                Full history
                <ArrowRight size={14} />
              </Button>
            </div>
          </Box>
        {:else}
          <Box class="flex min-h-32 flex-col items-center justify-center gap-3 rounded-2xl border border-base-800/80 bg-base-950/60 text-center">
            <History size={24} class="text-base-500" />
            <div class="space-y-1">
              <h3 class="text-sm font-semibold text-base-200">No syncs yet</h3>
              <p class="text-sm text-base-400">Run your first sync to see results here.</p>
            </div>
          </Box>
        {/if}
      </section>

      <!-- Quick actions -->
      <section>
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-base-400">Quick actions</h2>
        <div class="space-y-2">
          <Button variant="secondary" onclick={() => goto('/shows')} class="w-full justify-start">
            <Tv size={16} />
            Browse anime library
          </Button>
          <Button variant="secondary" onclick={() => goto('/activity')} class="w-full justify-start">
            <History size={16} />
            View sync activity
          </Button>
          <Button variant="secondary" onclick={() => goto('/settings')} class="w-full justify-start">
            <RefreshCw size={16} />
            Configure settings
          </Button>
        </div>
      </section>
    </div>

    <!-- Tracked anime preview -->
    {#if showCount > 0}
      <section>
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-base-400">Your anime</h2>
          <Button variant="ghost" size="sm" onclick={() => goto('/shows')}>
            View all
            <ArrowRight size={14} />
          </Button>
        </div>
        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each $shows.slice(0, 6) as show (show.id)}
            {@const stateBadge = syncStateBadge(show.enabled)}
            <Box class="cursor-pointer rounded-2xl border border-base-800/80 bg-base-950/60 p-4 transition-colors hover:border-base-700"
                 onclick={() => goto(`/shows/${show.id}`)}>
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 space-y-1">
                  <h3 class="truncate text-sm font-semibold text-base-50">{show.sonarr_title}</h3>
                  <p class="truncate text-xs text-base-400">{show.afl_title || show.afl_slug}</p>
                </div>
                <Badge variant={stateBadge.variant} size="sm">{stateBadge.label}</Badge>
              </div>
            </Box>
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>
