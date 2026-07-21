<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { getStatus } from '../lib/api.ts';
  import Waves from '@lucide/svelte/icons/waves';

  const navItems = [
    { path: '/shows', label: 'Library' },
    { path: '/activity', label: 'Activity' },
    { path: '/settings', label: 'Settings' },
  ];

  let sonarrConnected = $state(false);
  let lastSyncAt = $state<string | null>(null);
  let nextSyncAt = $state<string | null>(null);
  let now = $state(Date.now());
  const currentPath = $derived($page.url.pathname);

  onMount(() => {
    (async () => {
      try {
        const status = await getStatus();
        sonarrConnected = status?.status === 'ok';
        lastSyncAt = status?.last_sync_at ?? null;
        nextSyncAt = status?.next_sync_at ?? null;
      } catch {
        sonarrConnected = false;
      }
    })();
    const interval = setInterval(() => (now = Date.now()), 30000);
    return () => clearInterval(interval);
  });

  function isActive(itemPath: string, path: string) {
    if (itemPath === '/shows') {
      return path === '/shows' || path.startsWith('/shows/') || path === '/';
    }
    if (itemPath === '/activity') {
      return path === '/activity' || path === '/logs';
    }
    return path === itemPath;
  }

  function relativeFromNow(iso: string | null, currentNow: number): string {
    if (!iso) return '';
    const target = new Date(iso).getTime();
    const diff = target - currentNow;
    const abs = Math.abs(diff);
    const mins = Math.round(abs / 60000);
    if (mins < 1) return 'now';
    if (mins < 60) return `${mins}m`;
    const hours = Math.round(mins / 60);
    if (hours < 24) return `${hours}h`;
    const days = Math.round(hours / 24);
    return `${days}d`;
  }

  let lastSyncLabel = $derived(lastSyncAt ? relativeFromNow(lastSyncAt, now) : '');
  let nextSyncLabel = $derived(nextSyncAt ? relativeFromNow(nextSyncAt, now) : '');
</script>

<header class="sticky top-0 z-40 border-b border-rule bg-background/90 backdrop-blur-sm">
  <div class="flex h-[52px] items-center gap-5 px-5">
    <a href="/" class="flex items-center gap-2.5" aria-label="Mizu home">
      <span class="flex size-6 items-center justify-center rounded-md border border-rule font-mono text-[13px] text-primary">
        <Waves class="size-3.5" />
      </span>
      <strong class="text-sm font-semibold tracking-tight text-foreground">Mizu</strong>
    </a>

    <nav class="flex h-full items-center gap-0.5" aria-label="Primary">
      {#each navItems as item}
        {@const active = isActive(item.path, currentPath)}
        <a
          href={item.path}
          class="flex h-full items-center border-b-2 px-3.5 text-[13px] font-medium transition-colors -mb-px {active
            ? 'border-primary text-foreground'
            : 'border-transparent text-muted-foreground hover:text-foreground'}"
          aria-current={active ? 'page' : undefined}
        >
          {item.label}
        </a>
      {/each}
    </nav>

    <div class="ml-auto flex items-center font-mono text-[11px] text-muted-foreground">
      <span class="flex items-center gap-2">
        <span class="lamp {sonarrConnected ? '' : 'lamp-err'}"></span>
        Sonarr <b class="font-medium text-foreground">{sonarrConnected ? 'connected' : 'offline'}</b>
      </span>
      {#if lastSyncLabel}
        <span class="sep"></span>
        <span class="hidden items-center gap-1.5 sm:flex">
          last sync
          <b class="font-medium text-foreground tabular-nums">{lastSyncLabel}</b>
        </span>
      {/if}
      {#if nextSyncLabel}
        <span class="sep"></span>
        <span class="hidden items-center gap-1.5 sm:flex">
          next
          <b class="font-medium text-foreground tabular-nums">{nextSyncLabel}</b>
        </span>
      {/if}
    </div>
  </div>
</header>
