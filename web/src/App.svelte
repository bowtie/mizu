<script>
  import Router, { router, push } from 'svelte-spa-router';
  import wrap from 'svelte-spa-router/wrap';
  import Navbar from './components/Navbar.svelte';
  import Toast from './components/Toast.svelte';

  const routes = {
    '/shows': wrap({ asyncComponent: () => import('./pages/Shows.svelte') }),
    '/shows/:id': wrap({ asyncComponent: () => import('./pages/ShowDetail.svelte') }),
    '/activity': wrap({ asyncComponent: () => import('./pages/Activity.svelte') }),
    '/settings': wrap({ asyncComponent: () => import('./pages/Settings.svelte') }),
    '/logs': wrap({ asyncComponent: () => import('./pages/Activity.svelte') }),
  };

  $: if (router.location === '/') push('/shows');
</script>

<div class="min-h-screen bg-base-900 text-base-50">
  <Navbar />
  <main class="min-h-screen px-4 pb-28 pt-8 md:pb-8">
    <div class="mx-auto w-full max-w-5xl">
      <Router {routes} />
    </div>
  </main>
  <Toast />
</div>
