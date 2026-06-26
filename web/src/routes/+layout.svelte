<script>
  import '../app.css';
  import Navbar from '../components/Navbar.svelte';
  import Toast from '../components/Toast.svelte';

  let { children } = $props();
</script>

<div class="min-h-screen bg-base-900 text-base-50">
  <Navbar />
  <main class="min-h-screen px-4 pb-28 pt-8 md:pb-8">
    <div class="mx-auto w-full max-w-5xl">
      {@render children()}
    </div>
  </main>
  <Toast />
</div>
