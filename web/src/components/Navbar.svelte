<script>
  import { onMount } from 'svelte';
  import { location } from 'svelte-spa-router';
  import { getStatus } from '../lib/api.js';
  import IconHouseRegular from 'phosphor-icons-svelte/IconHouseRegular.svelte';
  import IconClockCounterClockwiseRegular from 'phosphor-icons-svelte/IconClockCounterClockwiseRegular.svelte';
  import IconGearSixRegular from 'phosphor-icons-svelte/IconGearSixRegular.svelte';
  import IconWaveSineRegular from 'phosphor-icons-svelte/IconWaveSineRegular.svelte';

  const navItems = [
    { path: '/shows', label: 'Shows', icon: IconHouseRegular },
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
  <div class="topbar mizu-topbar">
    <a class="brand-lockup" href="#/shows" aria-label="Mizu home">
      <span class="brand-mark"><IconWaveSineRegular size={18} weight="regular" /></span>
      <strong>Mizu</strong>
    </a>

    <nav class="topnav" aria-label="Primary">
      {#each navItems as item}
        <a href={`#${item.path}`} class="topnav-link" class:active={isActive(item.path, $location)}>
          <svelte:component this={item.icon} size={18} weight="regular" />
          <span>{item.label}</span>
        </a>
      {/each}
    </nav>

    <div class="topbar-utility">
      <span class="status-pill" class:offline={!sonarrConnected}>
        <span class="status-dot" class:online={sonarrConnected} class:offline={!sonarrConnected}></span>
        {sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
      </span>
    </div>
  </div>
</header>

<nav class="mobile-bottom-nav" aria-label="Primary mobile navigation">
  <div class="mobile-bottom-pill mizu-bottom-pill">
    {#each navItems as item}
      <a href={`#${item.path}`} class="mobile-nav-link" class:active={isActive(item.path, $location)}>
        <svelte:component this={item.icon} size={20} weight="regular" />
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
    padding: 18px 24px 0;
    pointer-events: none;
  }

  .topbar {
    max-width: 1220px;
    margin: 0 auto;
    padding: 12px 18px;
    display: grid;
    align-items: center;
    grid-template-columns: auto 1fr auto;
    gap: 22px;
    border-radius: 999px;
    pointer-events: auto;
  }

  .mizu-topbar {
    background: rgba(7, 16, 27, 0.9);
    border: 1px solid rgba(118, 167, 203, 0.16);
    box-shadow: 0 18px 52px rgba(0, 0, 0, 0.28);
    backdrop-filter: blur(18px);
  }

  .brand-lockup {
    display: inline-flex;
    align-items: center;
    gap: 12px;
    color: var(--text-primary);
  }

  .brand-lockup strong {
    font-family: 'Figtree', sans-serif;
    font-size: 1rem;
    color: var(--text-primary);
  }

  .brand-mark {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border-radius: 999px;
    background: linear-gradient(135deg, rgba(23, 118, 191, 0.36), rgba(35, 173, 221, 0.18));
    color: #ecf8ff;
  }

  .topnav {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 10px;
  }

  .topnav-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-width: 108px;
    padding: 10px 18px;
    border-radius: 999px;
    color: var(--text-secondary);
    font-family: 'Figtree', sans-serif;
    font-size: 14px;
    font-weight: 600;
    transition: background var(--transition), color var(--transition), box-shadow var(--transition);
  }

  .topnav-link:hover {
    color: var(--text-primary);
    background: rgba(255, 255, 255, 0.04);
  }

  .topnav-link.active {
    color: #f3fbff;
    background: rgba(18, 168, 216, 0.14);
    box-shadow: inset 0 0 0 1px rgba(118, 167, 203, 0.2);
  }

  .topbar-utility {
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-height: 38px;
    padding: 0 13px;
    border-radius: 999px;
    background: rgba(18, 168, 216, 0.12);
    border: 1px solid rgba(118, 167, 203, 0.16);
    color: #d8edf8;
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
  }

  .status-pill.offline {
    background: rgba(239, 68, 68, 0.14);
    border-color: rgba(248, 113, 113, 0.18);
    color: #fecaca;
  }

  .mobile-bottom-nav {
    display: none;
  }

  .mizu-bottom-pill {
    width: min(100%, 390px);
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 6px;
    padding: 8px;
    border-radius: 999px;
    background: rgba(6, 14, 24, 0.94);
    border: 1px solid rgba(118, 167, 203, 0.16);
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.34);
    backdrop-filter: blur(18px);
    pointer-events: auto;
  }

  .mobile-nav-link {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-height: 52px;
    padding: 6px 4px;
    border-radius: 999px;
    color: var(--text-secondary);
    font-size: 11px;
    font-weight: 600;
    text-align: center;
  }

  .mobile-nav-link.active {
    color: #f2fbff;
    background: rgba(18, 168, 216, 0.14);
  }

  @media (max-width: 900px) {
    .topbar {
      grid-template-columns: auto 1fr;
    }

    .topbar-utility {
      display: none;
    }

    .topnav {
      justify-content: flex-end;
      flex-wrap: wrap;
    }
  }

  @media (max-width: 720px) {
    .topbar-shell {
      display: none;
    }

    .mobile-bottom-nav {
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 160;
      display: flex;
      justify-content: center;
      padding: 0 14px calc(16px + env(safe-area-inset-bottom));
      pointer-events: none;
    }
  }
</style>



