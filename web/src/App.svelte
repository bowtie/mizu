<script>
  import Router from 'svelte-spa-router';
  import { location, push } from 'svelte-spa-router';
  import wrap from 'svelte-spa-router/wrap';
  import Navbar from './components/Navbar.svelte';
  import Toast from './components/Toast.svelte';

  let ShellEffect = null;

  const routes = {
    '/shows': wrap({ asyncComponent: () => import('./pages/Shows.svelte') }),
    '/shows/:id': wrap({ asyncComponent: () => import('./pages/ShowDetail.svelte') }),
    '/activity': wrap({ asyncComponent: () => import('./pages/Activity.svelte') }),
    '/settings': wrap({ asyncComponent: () => import('./pages/Settings.svelte') }),
    // Keep /logs as a compatibility alias for older links.
    '/logs': wrap({ asyncComponent: () => import('./pages/Activity.svelte') }),
  };

  $: if ($location === '/') {
    push('/shows');
  }

  $: isDetailRoute = /^\/shows\/[^/]+$/.test($location || '');
  $: hasShellEffect = ['/shows', '/activity', '/settings', '/logs'].includes($location || '');
  $: if (hasShellEffect && !isDetailRoute && !ShellEffect) {
    import('./components/ShellSpecularBand.svelte').then((module) => {
      ShellEffect = module.default;
    });
  }
</script>

<div class="app-shell">
  <div class="app-shell-backdrop" aria-hidden="true"></div>
  {#if hasShellEffect && !isDetailRoute && ShellEffect}
    <svelte:component this={ShellEffect} />
  {/if}
  <Navbar />
  <main class="main-content shell-main" class:detail-route-shell={isDetailRoute}>
    <Router {routes} />
  </main>
  <Toast />
</div>
