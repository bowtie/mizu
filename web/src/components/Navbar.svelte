<script>
  import { onMount } from 'svelte';
  import { location } from 'svelte-spa-router';
  import { getStatus } from '../lib/api.js';
  import { History, RadioTower, Settings as SettingsIcon, Tv, Waves } from 'lucide-svelte';

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
    <a class="brand-lockup" href="#/shows" aria-label="Mizu home">
      <span class="brand-mark"><Waves size={20} /></span>
      <strong>Mizu</strong>
    </a>

    <nav class="topnav" aria-label="Primary">
      {#each navItems as item}
        <a
          href={`#${item.path}`}
          class="topnav-link"
          class:active={isActive(item.path, $location)}
          class:tv-link={item.path === '/shows'}
        >
          <svelte:component this={item.icon} size={18} />
          <span>{item.label}</span>
        </a>
      {/each}
    </nav>

    <div class="topbar-utility">
      <span
        class="status-pill sonarr-pill"
        class:offline={!sonarrConnected}
        title={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
        aria-label={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
      >
        <RadioTower size={20} />
      </span>
    </div>
  </div>
</header>

<nav class="mobile-bottom-nav" aria-label="Primary mobile navigation">
  <div class="mobile-bottom-pill mizu-bottom-pill">
    {#each navItems as item}
      <a href={`#${item.path}`} class="mobile-nav-link" class:active={isActive(item.path, $location)}>
        <svelte:component this={item.icon} size={22} />
        <span>{item.label}</span>
      </a>
    {/each}
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
    padding: 10px 14px;
    display: grid;
    align-items: center;
    grid-template-columns: auto 1fr auto;
    gap: 18px;
    border-radius: 14px;
    pointer-events: auto;
    background: #111e30;
    box-shadow: var(--shadow-lg);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
  }

  .brand-lockup {
    display: inline-flex;
    align-items: center;
    gap: 9px;
    color: var(--text-primary);
    text-decoration: none;
  }

  .brand-mark {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 30px;
    border-radius: 8px;
    background: var(--blue-base);
    color: #fff;
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
    gap: 4px;
  }

  .topnav-link {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 8px 14px;
    border-radius: 9px;
    border: 0;
    color: var(--text-muted);
    font-size: var(--text-secondary-size);
    font-weight: var(--weight-semibold);
    line-height: 1;
    transition: background var(--transition), color var(--transition), box-shadow var(--transition);
  }

  .topnav-link.tv-link :global(svg) {
    display: block;
    transform: translateY(-1px);
  }

  .topnav-link:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
  }

  .topnav-link.active {
    color: #fff;
    background: var(--blue-base);
  }

  .topbar-utility {
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .status-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border-radius: 8px;
    background: var(--success-bg);
    color: var(--success);
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .status-pill.offline {
    background: var(--danger-bg);
    border-color: var(--danger-border);
    color: var(--danger);
  }

  .mobile-bottom-nav { display: none; }

  .mizu-bottom-pill {
    width: min(100%, 380px);
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
    padding: 8px;
    border-radius: 16px;
    background: #111e30;
    box-shadow: var(--shadow-lg);
    backdrop-filter: blur(16px);
    pointer-events: auto;
  }

  .mobile-nav-link {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 5px;
    min-height: 60px;
    padding: 8px 6px;
    border-radius: 11px;
    color: var(--text-muted);
    font-size: var(--text-caption);
    font-weight: var(--weight-semibold);
    letter-spacing: 0.03em;
    text-align: center;
    transition: background var(--transition), color var(--transition);
  }

  .mobile-nav-link.active {
    color: #fff;
    background: var(--blue-base);
  }

  .brand-mark :global(svg),
  .topnav-link :global(svg),
  .status-pill :global(svg) {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  .mobile-nav-link :global(svg) {
    width: 22px;
    height: 22px;
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
