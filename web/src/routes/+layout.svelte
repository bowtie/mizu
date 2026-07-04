<script>
  import '../app.css';
  import Navbar from '../components/Navbar.svelte';
  import Toast from '../components/Toast.svelte';
  import WarpBackground from '../components/WarpBackground.svelte';
  import StaticMeshBackground from '../components/StaticMeshBackground.svelte';
  import AmbientBackground from '../components/AmbientBackground.svelte';
  import GradientBackground from '../components/GradientBackground.svelte';
  import MeshBackground from '../components/MeshBackground.svelte';
  import WavesBackground from '../components/WavesBackground.svelte';
  import GodRaysBackground from '../components/GodRaysBackground.svelte';
  import MetaballsBackground from '../components/MetaballsBackground.svelte';
  import PerlinBackground from '../components/PerlinBackground.svelte';
  import SimplexBackground from '../components/SimplexBackground.svelte';
  import AuroraBackground from '../components/AuroraBackground.svelte';
  import DarkVeilBackground from '../components/DarkVeilBackground.svelte';
  import PaperBackground from '../components/PaperBackground.svelte';
  import { fade } from 'svelte/transition';
  import { page } from '$app/stores';

  let { children } = $props();
</script>

<div class="isolate min-h-screen text-foreground" data-sveltekit-preload-data="hover">
  <DarkVeilBackground speed={1} hueShift={30} />
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
