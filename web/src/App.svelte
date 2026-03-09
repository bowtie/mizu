<script>
  import Router from 'svelte-spa-router';
  import { location, push } from 'svelte-spa-router';
  import Navbar from './components/Navbar.svelte';
  import Toast from './components/Toast.svelte';

  import Shows from './pages/Shows.svelte';
  import ShowDetail from './pages/ShowDetail.svelte';
  import Activity from './pages/Activity.svelte';
  import Settings from './pages/Settings.svelte';

  const routes = {
    '/shows': Shows,
    '/shows/:id': ShowDetail,
    '/activity': Activity,
    '/settings': Settings,
    '/logs': Activity,
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

