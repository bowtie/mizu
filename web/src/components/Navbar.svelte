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
      <span
        class="status-pill sonarr-pill"
        class:offline={!sonarrConnected}
        title={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
        aria-label={sonarrConnected ? 'Sonarr online' : 'Sonarr offline'}
      >
        <IconBroadcastRegular size={16} weight="regular" />
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
    padding: 14px 22px 0;
    pointer-events: none;
  }

  .topbar {
    max-width: 1180px;
    margin: 0 auto;
    padding: 12px 16px;
    display: grid;
    align-items: center;
    grid-template-columns: auto 1fr auto;
    gap: 18px;
    border-radius: 16px;
    pointer-events: auto;
  }

  .mizu-topbar {
    background: rgba(8, 25, 45, 0.84);
    border: 1px solid var(--border);
    box-shadow: 0 8px 22px rgba(0, 0, 0, 0.16);
    backdrop-filter: blur(12px);
  }

  .brand-lockup {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    color: var(--text-primary);
  }

  .brand-lockup strong {
    font-family: 'Figtree', sans-serif;
    font-size: 0.98rem;
    color: var(--text-primary);
  }

  .brand-mark {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: 10px;
    background: rgba(108, 167, 195, 0.12);
    color: var(--accent-cream);
    border: 1px solid rgba(108, 167, 195, 0.16);
  }

  .topnav {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 8px;
  }

  .topnav-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-width: 92px;
    padding: 8px 12px;
    border-radius: 10px;
    border: 1px solid transparent;
    color: var(--text-secondary);
    font-family: 'Figtree', sans-serif;
    font-size: 13px;
    font-weight: 600;
    transition: background var(--transition), color var(--transition), border-color var(--transition);
  }

  .topnav-link:hover {
    color: var(--text-primary);
    background: rgba(252, 250, 242, 0.03);
  }

  .topnav-link.active {
    color: var(--text-primary);
    background: rgba(108, 167, 195, 0.1);
    border-color: rgba(108, 167, 195, 0.14);
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
    border-radius: 10px;
    border: 1px solid rgba(131, 201, 147, 0.26);
    background: var(--success-bg);
    color: var(--success);
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .status-pill.offline {
    background: var(--danger-bg);
    border-color: rgba(217, 119, 110, 0.24);
    color: var(--danger);
  }

  .mobile-bottom-nav {
    display: none;
  }

  .mizu-bottom-pill {
    width: min(100%, 410px);
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
    padding: 10px;
    border-radius: 18px;
    background: rgba(8, 25, 45, 0.92);
    border: 1px solid var(--border);
    box-shadow: 0 8px 22px rgba(0, 0, 0, 0.18);
    backdrop-filter: blur(12px);
    pointer-events: auto;
  }

  .mobile-nav-link {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-height: 68px;
    padding: 9px 6px;
    border-radius: 14px;
    color: var(--text-secondary);
    font-size: 12px;
    font-weight: 700;
    text-align: center;
  }

  .mobile-nav-link.active {
    color: var(--text-primary);
    background: rgba(108, 167, 195, 0.1);
  }

  @media (max-width: 1040px) {
    .topbar {
      padding: 10px 14px;
      gap: 14px;
    }

    .topnav-link {
      min-width: 82px;
      padding: 8px 10px;
    }
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
      gap: 6px;
    }

    .topnav-link {
      min-width: 76px;
      font-size: 12px;
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
      padding: 0 14px calc(12px + env(safe-area-inset-bottom));
      pointer-events: none;
    }
  }
</style>
