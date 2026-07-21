<script>
  import '../app.css';
  import Navbar from '../components/Navbar.svelte';
  import Toast from '../components/Toast.svelte';
  import { fade } from 'svelte/transition';
  import { page } from '$app/stores';

  let { children } = $props();
</script>

<div class="isolate min-h-[100dvh] bg-background text-foreground" data-sveltekit-preload-data="hover">
  <Navbar />
  <main class="min-h-[100dvh]">
    <div class="mx-auto w-full max-w-[1080px] px-6 pb-20 pt-6">
      {#key $page.url.pathname}
        <div in:fade={{ duration: 200, delay: 50 }}>
          {@render children()}
        </div>
      {/key}
    </div>
  </main>
  <Toast />
</div>
