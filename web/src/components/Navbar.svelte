<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';
  import { getStatus } from '../lib/api.js';
  import History from '@lucide/svelte/icons/history';
  import LayoutDashboard from '@lucide/svelte/icons/layout-dashboard';
  import RadioTower from '@lucide/svelte/icons/radio-tower';
  import SettingsIcon from '@lucide/svelte/icons/settings';
  import Tv from '@lucide/svelte/icons/tv';
  import Waves from '@lucide/svelte/icons/waves';

  const navItems = [
    { path: '/', label: 'Dashboard', icon: LayoutDashboard },
    { path: '/shows', label: 'Anime', icon: Tv },
    { path: '/activity', label: 'Activity', icon: History },
    { path: '/settings', label: 'Settings', icon: SettingsIcon },
  ];

  let sonarrConnected = false;
  $: sonarrStatusLabel = sonarrConnected ? 'Sonarr connection is healthy.' : 'Sonarr is offline or unreachable.';
  $: currentPath = $page.url.pathname;

  onMount(async () => {
    try {
      const status = await getStatus();
      sonarrConnected = status?.status === 'ok';
    } catch {
      sonarrConnected = false;
    }
  });

  function isActive(itemPath: string, path: string) {
    if (itemPath === '/') {
      return path === '/';
    }
    if (itemPath === '/shows') {
      return path === '/shows' || path.startsWith('/shows/');
    }
    if (itemPath === '/activity') {
      return path === '/activity' || path === '/logs';
    }
    return path === itemPath;
  }
</script>

<header class="sticky top-4 z-40 hidden px-4 md:block">
  <div class="mx-auto flex max-w-6xl flex-col rounded-2xl border border-border bg-popover/82 px-4 shadow-[0_10px_30px_rgba(2,6,23,0.28)] backdrop-blur-xl">
    <div class="flex min-h-16 items-center justify-between gap-4 py-3">
      <a href="/" class="flex min-w-0 items-center gap-3" aria-label="Mizu home">
        <span class="flex size-9 items-center justify-center rounded-lg border border-primary/20 bg-primary/10 text-primary">
          <Waves class="size-[18px]" />
        </span>
        <strong class="block truncate text-sm font-semibold tracking-tight text-foreground">Mizu</strong>
      </a>

      <nav class="hidden items-center gap-1 md:flex" aria-label="Primary">
        {#each navItems as item}
          {@const Icon = item.icon}
          <Button
            href={item.path}
            variant={isActive(item.path, currentPath) ? 'secondary' : 'ghost'}
            class="rounded-lg border-transparent"
            aria-current={isActive(item.path, currentPath) ? 'page' : undefined}
          >
            <Icon class="size-4" />
            {item.label}
          </Button>
        {/each}
      </nav>

      <Tooltip.Provider delayDuration={120}>
        <Tooltip.Root>
          <Tooltip.Trigger>
            <Badge
              variant={sonarrConnected ? 'success' : 'destructive'}
              class="size-10 shrink-0 cursor-help justify-center rounded-xl px-0"
              aria-label={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
            >
              <RadioTower class="size-4" />
            </Badge>
          </Tooltip.Trigger>
          <Tooltip.Content side="bottom" align="center" sideOffset={10}>
            {sonarrStatusLabel}
          </Tooltip.Content>
        </Tooltip.Root>
      </Tooltip.Provider>
    </div>
  </div>
</header>

<div class="fixed inset-x-0 bottom-4 z-40 px-4 md:hidden">
  <nav
    class="mx-auto flex max-w-sm items-center gap-2 rounded-2xl border border-border bg-popover/88 p-2 shadow-[0_10px_30px_rgba(2,6,23,0.28)] backdrop-blur-xl"
    aria-label="Primary mobile"
  >
    {#each navItems as item}
      {@const Icon = item.icon}
      <Button
        href={item.path}
        variant={isActive(item.path, currentPath) ? 'secondary' : 'ghost'}
        class="min-w-0 flex-1 justify-center rounded-xl border-transparent whitespace-nowrap"
        aria-current={isActive(item.path, currentPath) ? 'page' : undefined}
      >
        <Icon class="size-4" />
        {item.label}
      </Button>
    {/each}
  </nav>
</div>
