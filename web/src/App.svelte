<script>
  import Router from 'svelte-spa-router';
  import { location, push } from 'svelte-spa-router';
  import wrap from 'svelte-spa-router/wrap';
  import Navbar from './components/Navbar.svelte';
  import Toast from './components/Toast.svelte';

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
</script>

<div class="app-shell">
  <Navbar />
  <main class="main-content shell-main">
    <Router {routes} />
  </main>
  <Toast />
</div>
