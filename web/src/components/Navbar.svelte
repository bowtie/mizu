<script>
  import { onMount } from 'svelte';
  import { location } from 'svelte-spa-router';
  import { getStatus } from '../lib/api.js';
  import IconTelevisionSimpleRegular from 'phosphor-icons-svelte/IconTelevisionSimpleRegular.svelte';
  import IconClockCounterClockwiseRegular from 'phosphor-icons-svelte/IconClockCounterClockwiseRegular.svelte';
  import IconGearSixRegular from 'phosphor-icons-svelte/IconGearSixRegular.svelte';
  import IconWaveSineRegular from 'phosphor-icons-svelte/IconWaveSineRegular.svelte';
  import IconBroadcastRegular from 'phosphor-icons-svelte/IconBroadcastRegular.svelte';

  const navItems = [
    { path: '/shows', label: 'Anime', icon: IconTelevisionSimpleRegular },
    { path: '/activity', label: 'Activity', icon: IconClockCounterClockwiseRegular },
    { path: '/settings', label: 'Settings', icon: IconGearSixRegular },
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
      <span class="brand-mark"><IconWaveSineRegular size={20} weight="regular" /></span>
      <strong>Mizu</strong>
    </a>

    <nav class="topnav" aria-label="Primary">
      {#each navItems as item}
        <a href={`#${item.path}`} class="topnav-link" class:active={isActive(item.path, $location)}>
          <svelte:component this={item.icon} size={20} weight="regular" />
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
        <IconBroadcastRegular size={20} weight="regular" />
      </span>
    </div>
  </div>
</header>

<nav class="mobile-bottom-nav" aria-label="Primary mobile navigation">
  <div class="mobile-bottom-pill mizu-bottom-pill">
    {#each navItems as item}
      <a href={`#${item.path}`} class="mobile-nav-link" class:active={isActive(item.path, $location)}>
        <svelte:component this={item.icon} size={28} weight="regular" />
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
    padding: 8px 12px;
    display: grid;
    align-items: center;
    grid-template-columns: auto 1fr auto;
    gap: 16px;
    border-radius: 14px;
    pointer-events: auto;
    background: rgba(5, 13, 26, 0.82);
    border: 1px solid var(--border);
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
    font-family: 'Figtree', sans-serif;
    font-size: 1rem;
    font-weight: 800;
    letter-spacing: -0.01em;
    color: var(--text-primary);
  }

  .topnav {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 2px;
  }

  .topnav-link {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 7px 14px;
    border-radius: 9px;
    border: 1px solid transparent;
    color: var(--text-muted);
    font-family: 'Figtree', sans-serif;
    font-size: 13px;
    font-weight: 600;
    transition: background var(--transition), color var(--transition), border-color var(--transition);
  }

  .topnav-link:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .topnav-link.active {
    color: #fff;
    background: var(--blue-base);
    border-color: var(--accent-border);
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
    width: 32px;
    height: 32px;
    border-radius: 8px;
    border: 1px solid var(--success-border);
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
    gap: 6px;
    padding: 7px;
    border-radius: 16px;
    background: rgba(5, 13, 26, 0.92);
    border: 1px solid var(--border);
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
    min-height: 58px;
    padding: 8px 6px;
    border-radius: 11px;
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 700;
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








