<script>
  import { onMount } from 'svelte';
  import { location } from 'svelte-spa-router';
  import { getStatus } from '../lib/api.js';
  import { History, RadioTower, Settings as SettingsIcon, Tv, Waves } from 'lucide-svelte';
  import Surface from './ui/Surface.svelte';

  const navItems = [
    { path: '/shows', label: 'Anime', icon: Tv },
    { path: '/activity', label: 'Activity', icon: History },
    { path: '/settings', label: 'Settings', icon: SettingsIcon },
  ];

  let sonarrConnected = false;

  onMount(async () => {
    try {
      const status = await getStatus();
      sonarrConnected = status?.status === 'ok';
    } catch {
      sonarrConnected = false;
    }
  });

  function isActive(itemPath, currentPath) {
    if (itemPath === '/shows') {
      return currentPath === '/shows' || currentPath.startsWith('/shows/');
    }
    if (itemPath === '/activity') {
      return currentPath === '/activity' || currentPath === '/logs';
    }
    return currentPath === itemPath;
  }
</script>

<header class="topbar-shell">
  <div class="topbar">
    <Surface class="topbar-surface" variant="muted" padding="sm" blur>
      <a class="brand-lockup" href="#/shows" aria-label="Mizu home">
        <span class="brand-mark"><Waves size={18} /></span>
        <strong>Mizu</strong>
      </a>

      <nav class="topnav" aria-label="Primary">
        {#each navItems as item}
        <a
          href={`#${item.path}`}
          class="topnav-link"
          class:active={isActive(item.path, $location)}
          class:tv-link={item.path === '/shows'}
          aria-current={isActive(item.path, $location) ? 'page' : undefined}
        >
          <svelte:component this={item.icon} size={16} />
          <span>{item.label}</span>
          </a>
        {/each}
      </nav>

      <div class="topbar-utility">
        <span
          class="status-indicator"
          class:offline={!sonarrConnected}
          title={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
          aria-label={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
        >
          <RadioTower size={16} />
        </span>
      </div>
    </Surface>
  </div>
</header>

<nav class="mobile-bottom-nav" aria-label="Primary mobile navigation">
  <div class="mobile-bottom-bar">
    <Surface class="mobile-bottom-bar-surface" variant="muted" padding="sm" blur>
      {#each navItems as item}
        <a
          href={`#${item.path}`}
          class="mobile-nav-link"
          class:active={isActive(item.path, $location)}
          aria-current={isActive(item.path, $location) ? 'page' : undefined}
        >
          <svelte:component this={item.icon} size={18} />
          <span>{item.label}</span>
        </a>
      {/each}
    </Surface>
  </div>
</nav>

<style>
  .topbar-shell {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 140;
    padding: 12px 20px 0;
    pointer-events: none;
  }

  .topbar {
    max-width: var(--nav-max);
    margin: 0 auto;
    pointer-events: auto;
  }

  .topbar :global(.topbar-surface) {
    display: grid;
    align-items: center;
    grid-template-columns: auto 1fr auto;
    gap: 16px;
  }

  .brand-lockup {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    color: var(--text-primary);
    text-decoration: none;
  }

  .brand-mark {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 7px;
    background: color-mix(in srgb, var(--surface-soft) 65%, rgba(255, 255, 255, 0.04) 35%);
    border: 1px solid color-mix(in srgb, var(--border) 74%, rgba(255, 255, 255, 0.08) 26%);
    color: var(--accent);
    flex-shrink: 0;
  }

  .brand-lockup strong {
    font-family: var(--font-heading);
    font-size: 1rem;
    font-weight: var(--weight-bold);
    letter-spacing: var(--tracking-tight);
    color: var(--text-primary);
  }

  .topnav {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 6px;
  }

  .topnav-link {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-height: 36px;
    padding: 0 12px;
    border-radius: 8px;
    border: 1px solid transparent;
    color: var(--text-muted);
    font-size: var(--text-secondary-size);
    font-weight: var(--weight-semibold);
    line-height: 1;
    transition: background var(--transition), color var(--transition), border-color var(--transition);
  }

  .topnav-link.tv-link :global(svg) {
    display: block;
    transform: translateY(-1px);
  }

  .topnav-link:hover {
    color: var(--text-primary);
    background: color-mix(in srgb, var(--surface-hover) 72%, rgba(255, 255, 255, 0.02) 28%);
    border-color: var(--divider-border);
  }

  .topnav-link.active {
    color: var(--text-primary);
    background: color-mix(in srgb, var(--surface-soft) 82%, rgba(255, 255, 255, 0.03) 18%);
    border-color: var(--accent-border);
  }

  .topbar-utility {
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .status-indicator {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border-radius: 8px;
    border: 1px solid var(--success-border);
    background: var(--success-bg);
    color: var(--success);
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .status-indicator.offline {
    background: var(--danger-bg);
    border-color: var(--danger-border);
    color: var(--danger);
  }

  .mobile-bottom-nav { display: none; }

  .mobile-bottom-bar {
    width: min(100%, 380px);
    pointer-events: auto;
  }

  .mobile-bottom-bar :global(.mobile-bottom-bar-surface) {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 6px;
  }

  .mobile-nav-link {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-height: 54px;
    padding: 7px 6px;
    border-radius: 8px;
    border: 1px solid transparent;
    color: var(--text-muted);
    font-size: var(--text-caption);
    font-weight: var(--weight-semibold);
    text-align: center;
    transition: background var(--transition), color var(--transition), border-color var(--transition);
  }

  .mobile-nav-link:hover {
    color: var(--text-primary);
    background: color-mix(in srgb, var(--surface-hover) 72%, rgba(255, 255, 255, 0.02) 28%);
    border-color: var(--divider-border);
  }

  .mobile-nav-link.active {
    color: var(--text-primary);
    background: color-mix(in srgb, var(--surface-soft) 82%, rgba(255, 255, 255, 0.03) 18%);
    border-color: var(--accent-border);
  }

  .brand-mark :global(svg),
  .topnav-link :global(svg),
  .status-indicator :global(svg) {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  .mobile-nav-link :global(svg) {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
  }

  @media (max-width: 900px) {
    .topbar { gap: 12px; }
    .topbar-utility { display: none; }
    .topnav { justify-content: flex-end; }
  }

  @media (max-width: 720px) {
    .topbar-shell { display: none; }

    .mobile-bottom-nav {
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 160;
      display: flex;
      justify-content: center;
      padding: 0 14px calc(10px + env(safe-area-inset-bottom));
      pointer-events: none;
    }
  }
</style>
