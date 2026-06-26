<script>
  import { onMount } from 'svelte';
  import { router } from 'svelte-spa-router';
  import { Badge, Button, Tooltip } from '@foxui/core';
  import { getStatus } from '../lib/api.js';
  import { History, LayoutDashboard, RadioTower, Settings as SettingsIcon, Tv, Waves } from 'lucide-svelte';

  const navItems = [
    { path: '/', label: 'Dashboard', icon: LayoutDashboard },
    { path: '/shows', label: 'Anime', icon: Tv },
    { path: '/activity', label: 'Activity', icon: History },
    { path: '/settings', label: 'Settings', icon: SettingsIcon },
  ];

  let sonarrConnected = false;
  $: sonarrStatusLabel = sonarrConnected ? 'Sonarr connection is healthy.' : 'Sonarr is offline or unreachable.';

  onMount(async () => {
    try {
      const status = await getStatus();
      sonarrConnected = status?.status === 'ok';
    } catch {
      sonarrConnected = false;
    }
  });

  function isActive(itemPath, currentPath) {
    if (itemPath === '/') {
      return currentPath === '/';
    }
    if (itemPath === '/shows') {
      return currentPath === '/shows' || currentPath.startsWith('/shows/');
    }
    if (itemPath === '/activity') {
      return currentPath === '/activity' || currentPath === '/logs';
    }
    return currentPath === itemPath;
  }
</script>

<header class="sticky top-4 z-40 hidden px-4 md:block">
  <div class="mx-auto flex max-w-6xl flex-col rounded-2xl border border-base-800/80 bg-base-950/82 px-4 shadow-[0_10px_30px_rgba(2,6,23,0.28)] backdrop-blur-xl">
    <div class="flex min-h-16 items-center justify-between gap-4 py-3">
      <a href="#/" class="flex min-w-0 items-center gap-3" aria-label="Mizu home">
        <span class="flex size-9 items-center justify-center rounded-lg border border-blue-500/20 bg-blue-950/30 text-blue-300">
          <Waves size={18} />
        </span>
        <strong class="block truncate text-sm font-semibold tracking-tight text-base-50">Mizu</strong>
      </a>

      <nav class="hidden items-center gap-1 md:flex" aria-label="Primary">
        {#each navItems as item}
          <Button
            href={`#${item.path}`}
            variant={isActive(item.path, router.location) ? 'secondary' : 'ghost'}
            size="default"
            class="rounded-lg border-transparent"
            aria-current={isActive(item.path, router.location) ? 'page' : undefined}
          >
            <svelte:component this={item.icon} size={16} />
            {item.label}
          </Button>
        {/each}
      </nav>

      <Tooltip
        withContext
        delayDuration={120}
        text={sonarrStatusLabel}
        contentProps={{ side: 'bottom', align: 'center', sideOffset: 10 }}
      >
        {#snippet child({ props })}
          <Badge
            {...props}
            variant={sonarrConnected ? 'green' : 'rose'}
            size="md"
            class="size-10 shrink-0 cursor-help justify-center rounded-xl px-0"
            aria-label={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
          >
            <RadioTower size={16} />
          </Badge>
        {/snippet}
      </Tooltip>
    </div>
  </div>
</header>

<div class="fixed inset-x-0 bottom-4 z-40 px-4 md:hidden">
  <nav
    class="mx-auto flex max-w-sm items-center gap-2 rounded-2xl border border-base-800/80 bg-base-950/88 p-2 shadow-[0_10px_30px_rgba(2,6,23,0.28)] backdrop-blur-xl"
    aria-label="Primary mobile"
  >
    {#each navItems as item}
      <Button
        href={`#${item.path}`}
        variant={isActive(item.path, router.location) ? 'secondary' : 'ghost'}
        size="default"
        class="min-w-0 flex-1 justify-center rounded-xl border-transparent whitespace-nowrap"
        aria-current={isActive(item.path, router.location) ? 'page' : undefined}
      >
        <svelte:component this={item.icon} size={16} />
        {item.label}
      </Button>
    {/each}
  </nav>
</div>
