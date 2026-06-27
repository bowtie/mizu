<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import Box from '$lib/components/Box.svelte';
  import { shows, syncStatus, settings, addNotification } from '../lib/stores.js';
  import { getShows, getStats, getSyncLogs, triggerSync, getSettings, getStatus } from '../lib/api.js';
  import { runStatusBadge, syncStateBadge } from '../lib/fox.js';
  import { resolveSonarrImageUrl } from '../lib/sonarr-images.js';
  import {
    ArrowRight,
    CircleAlert,
    CircleCheckBig,
    History,
    LoaderCircle,
    RadioTower,
    RefreshCw,
    Settings as SettingsIcon,
    TriangleAlert,
    Tv,
    Waves,
  } from 'lucide-svelte';

  let loading = $state(true);
  let syncing = $state(false);
  let stats = $state(null);
  let recentLogs = $state([]);
  let connected = $state(false);

  onMount(async () => {
    await Promise.all([loadDashboard(), loadShows(), loadSettings(), loadStatus()]);
  });

  async function loadDashboard() {
    try {
      const [statsRes, logsRes] = await Promise.all([
        getStats(),
        getSyncLogs(5, 0),
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
    } catch (err) {
      addNotification('Failed to load shows: ' + err.message, 'error');
    }
  }

  async function loadSettings() {
    try {
      const data = await getSettings();
      settings.set(data);
    } catch (err) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
  }

  async function loadStatus() {
    try {
      const data = await getStatus();
      connected = data?.status === 'ok';
      syncStatus.update((s) => ({ ...s, connected }));
    } catch (err) {
      connected = false;
      syncStatus.update((s) => ({ ...s, connected: false }));
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

  function timeAgo(dt) {
    if (!dt) return 'never';
    const diff = Date.now() - new Date(dt).getTime();
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return `${Math.floor(diff / 86400000)}d ago`;
  }

  function posterUrl(show) {
    if (!show?.sonarr_images?.length) return '';
    const preferred = show.sonarr_images.find((i) => i.coverType === 'poster') || show.sonarr_images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, $settings?.sonarr_url || '');
  }

  let showCount = $derived($shows.length);
  let pausedCount = $derived(($shows || []).filter((s) => !s.enabled).length);
  let lastSync = $derived(recentLogs[0] || null);
  let failedSyncs = $derived(recentLogs.filter((l) => l.status === 'failed' || l.status === 'partial'));
  let needsAttention = $derived(failedSyncs.length > 0 || pausedCount > 0);

  const statCards = [
    { label: 'Anime tracked', key: 'shows', icon: Tv, accent: 'text-primary', tint: 'border-primary/20 bg-primary/5' },
    { label: 'Total episodes', key: 'total', icon: Waves, accent: 'text-primary', tint: 'border-primary/20 bg-primary/5' },
    { label: 'Filler managed', key: 'filler', icon: CircleCheckBig, accent: 'text-destructive', tint: 'border-destructive/20 bg-destructive/5' },
    { label: 'Canon episodes', key: 'canon', icon: CircleCheckBig, accent: 'text-success', tint: 'border-success/20 bg-success/5' },
  ];
</script>

<svelte:head>
  <title>Mizu - Dashboard</title>
</svelte:head>

<div class="space-y-8">
  <!-- Hero: status + sync -->
  <section class="relative overflow-hidden rounded-2xl border border-border bg-gradient-to-br from-card via-card to-primary/5 p-6 md:p-8">
    <div class="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
      <div class="space-y-3">
        <div class="flex items-center gap-2">
          <span class="flex size-10 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
            <Waves size={22} />
          </span>
          <div>
            <h1 class="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">Mizu</h1>
            <p class="text-sm text-muted-foreground">
              {#if showCount > 0}
                Tracking {showCount} anime · {stats?.total || 0} episodes
              {:else}
                No anime tracked yet
              {/if}
            </p>
          </div>
        </div>

        <!-- Connection + last sync inline -->
        <div class="flex flex-wrap items-center gap-3 text-sm">
          <span class="flex items-center gap-1.5 {connected ? 'text-success' : 'text-destructive'}">
            <RadioTower size={14} />
            {connected ? 'Sonarr connected' : 'Sonarr offline'}
          </span>
          {#if lastSync}
            <span class="text-muted-foreground">·</span>
            <span class="text-muted-foreground">
              Last sync {timeAgo(lastSync.started_at)}
            </span>
          {/if}
        </div>
      </div>

      <div class="flex gap-3">
        <Button variant="secondary" onclick={() => goto('/shows')}>
          <Tv size={16} />
          Browse anime
        </Button>
        <Button variant="default" onclick={handleSync} disabled={syncing}>
          {#if syncing}
            <LoaderCircle size={16} class="animate-spin" />
          {:else}
            <RefreshCw size={16} />
          {/if}
          {syncing ? 'Syncing...' : 'Sync now'}
        </Button>
      </div>
    </div>
  </section>

  {#if loading}
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-2xl border border-border bg-popover/60 text-center">
      <LoaderCircle size={30} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">Loading dashboard</h2>
        <p class="text-sm text-muted-foreground">Gathering your library overview.</p>
      </div>
    </Box>
  {:else}
    <!-- Needs attention -->
    {#if needsAttention}
      <section class="space-y-3">
        <h2 class="flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          <CircleAlert size={16} class="text-warning" />
          Needs attention
        </h2>
        <div class="grid gap-4 sm:grid-cols-2">
          {#if failedSyncs.length > 0}
            <Box class="rounded-2xl border border-destructive/20 bg-destructive/10 p-5">
              <div class="flex items-start gap-3">
                <span class="flex size-10 shrink-0 items-center justify-center rounded-xl border border-destructive/30 bg-destructive/10 text-destructive">
                  <TriangleAlert size={20} />
                </span>
                <div class="space-y-1.5">
                  <h3 class="text-sm font-semibold text-foreground">
                    {failedSyncs.length} {failedSyncs.length === 1 ? 'sync run needs review' : 'sync runs need review'}
                  </h3>
                  <p class="text-sm text-muted-foreground">Recent syncs completed with errors or partial results.</p>
                  <Button variant="ghost" size="sm" onclick={() => goto('/activity')} class="mt-1 -ml-2">
                    View activity
                    <ArrowRight size={14} />
                  </Button>
                </div>
              </div>
            </Box>
          {/if}
          {#if pausedCount > 0}
            <Box class="rounded-2xl border border-border bg-popover/60 p-5">
              <div class="flex items-start gap-3">
                <span class="flex size-10 shrink-0 items-center justify-center rounded-xl border border-border bg-card text-muted-foreground">
                  <Waves size={20} />
                </span>
                <div class="space-y-1.5">
                  <h3 class="text-sm font-semibold text-foreground">
                    {pausedCount} {pausedCount === 1 ? 'anime paused' : 'anime paused'}
                  </h3>
                  <p class="text-sm text-muted-foreground">Paused anime are not being synced automatically.</p>
                  <Button variant="ghost" size="sm" onclick={() => goto('/shows')} class="mt-1 -ml-2">
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
      <section class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {#each statCards as card}
          <Box class="rounded-2xl border {card.tint} p-5">
            <div class="flex flex-col gap-3">
              <span class="flex size-11 shrink-0 items-center justify-center rounded-xl border border-border bg-card {card.accent}">
                <card.icon size={20} />
              </span>
              <div>
                <p class="text-2xl font-bold tabular-nums text-foreground">
                  {(card.key === 'shows' ? showCount : stats[card.key] || 0).toLocaleString()}
                </p>
                <p class="text-sm text-muted-foreground">{card.label}</p>
              </div>
            </div>
          </Box>
        {/each}
      </section>
    {/if}

    <!-- Recent activity + Quick actions -->
    <div class="grid gap-6 lg:grid-cols-3">
      <!-- Recent activity feed -->
      <section class="lg:col-span-2">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Recent activity</h2>
          <Button variant="ghost" size="sm" onclick={() => goto('/activity')}>
            Full history
            <ArrowRight size={14} />
          </Button>
        </div>
        <div class="space-y-3">
          {#if recentLogs.length > 0}
            {#each recentLogs as log (log.id)}
              {@const logBadge = runStatusBadge(log.status)}
              <Box class="rounded-2xl border border-border bg-popover/60 p-4">
                <div class="flex items-center justify-between gap-4">
                  <div class="flex items-center gap-3">
                    <span class="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-card {logBadge.variant === 'success' ? 'text-success' : logBadge.variant === 'destructive' ? 'text-destructive' : 'text-warning'}">
                      {#if log.status === 'success'}
                        <CircleCheckBig size={16} />
                      {:else if log.status === 'failed'}
                        <TriangleAlert size={16} />
                      {:else}
                        <History size={16} />
                      {/if}
                    </span>
                    <div class="min-w-0 space-y-0.5">
                      <div class="flex items-center gap-2">
                        <Badge variant={logBadge.variant}>{logBadge.label}</Badge>
                        <span class="text-xs text-muted-foreground">{formatDuration(log.started_at, log.completed_at)}</span>
                      </div>
                      <p class="truncate text-sm text-foreground">{humanSummary(log)}</p>
                      <p class="text-xs text-muted-foreground">{formatTime(log.started_at)}</p>
                    </div>
                  </div>
                </div>
              </Box>
            {/each}
          {:else}
            <Box class="flex min-h-32 flex-col items-center justify-center gap-3 rounded-2xl border border-border bg-popover/60 text-center">
              <History size={24} class="text-muted-foreground" />
              <div class="space-y-1">
                <h3 class="text-sm font-semibold text-muted-foreground">No syncs yet</h3>
                <p class="text-sm text-muted-foreground">Run your first sync to see results here.</p>
              </div>
            </Box>
          {/if}
        </div>
      </section>

      <!-- Quick actions -->
      <section>
        <h2 class="mb-4 text-sm font-semibold uppercase tracking-wide text-muted-foreground">Quick actions</h2>
        <div class="space-y-3">
          <Button variant="secondary" onclick={() => goto('/shows')} class="w-full justify-start">
            <Tv size={16} />
            Browse anime library
          </Button>
          <Button variant="secondary" onclick={() => goto('/activity')} class="w-full justify-start">
            <History size={16} />
            View sync activity
          </Button>
          <Button variant="secondary" onclick={() => goto('/settings')} class="w-full justify-start">
            <SettingsIcon size={16} />
            Configure settings
          </Button>
        </div>
      </section>
    </div>

    <!-- Tracked anime preview with posters -->
    {#if showCount > 0}
      <section>
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Your anime</h2>
          <Button variant="ghost" size="sm" onclick={() => goto('/shows')}>
            View all
            <ArrowRight size={14} />
          </Button>
        </div>
        <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
          {#each $shows.slice(0, 6) as show (show.id)}
            {@const stateBadge = syncStateBadge(show.enabled)}
            {@const poster = posterUrl(show)}
            <a
              href={`/shows/${show.id}`}
              class="group space-y-2"
            >
              <div class="poster-frame relative aspect-[2/3] overflow-hidden rounded-xl border border-border bg-card/80 transition-colors group-hover:border-primary/40">
                {#if poster}
                  <img
                    src={poster}
                    alt={show.sonarr_title}
                    class="poster-image h-full w-full object-cover opacity-0 transition-all duration-500 ease-in-out group-hover:scale-105"
                    onload={(e) => { e.currentTarget.style.opacity = '1'; }}
                    onerror={(e) => { e.currentTarget.style.display = 'none'; e.currentTarget.parentElement.dataset.posterState = 'failed'; }}
                  />
                {/if}
                <div class="poster-skeleton absolute inset-0"></div>
                <div class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-popover/95 to-transparent p-2 pt-8">
                  <Badge variant={stateBadge.variant} class="text-[0.625rem]">{stateBadge.label}</Badge>
                </div>
              </div>
              <div class="space-y-0.5">
                <h3 class="truncate text-xs font-semibold text-foreground">{show.sonarr_title}</h3>
                <p class="truncate text-[0.6875rem] text-muted-foreground">{show.afl_title || show.afl_slug || ''}</p>
              </div>
            </a>
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>
