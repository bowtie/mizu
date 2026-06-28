<script>
  import '../app.css';
  import Navbar from '../components/Navbar.svelte';
  import Toast from '../components/Toast.svelte';
  import { fade } from 'svelte/transition';
  import { page } from '$app/stores';

  let { children } = $props();
</script>

<div class="dark min-h-screen bg-background text-foreground" data-sveltekit-preload-data="hover">
  <Navbar />
  <main class="min-h-screen px-6 pb-32 pt-10 md:px-8 md:pb-12">
    <div class="mx-auto w-full max-w-5xl">
      {#key $page.url.pathname}
        <div in:fade={{ duration: 200, delay: 50 }}>
          {@render children()}
        </div>
      {/key}
    </div>
  </main>
  <Toast />
</div>
