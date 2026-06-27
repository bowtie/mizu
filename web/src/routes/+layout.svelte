<script>
  import '../app.css';
  import Navbar from '../components/Navbar.svelte';
  import Toast from '../components/Toast.svelte';

  let { children } = $props();
</script>

<div class="dark min-h-screen bg-background text-foreground">
  <Navbar />
  <main class="min-h-screen px-4 pb-32 pt-8 md:pb-10">
    <div class="mx-auto w-full max-w-5xl">
      {@render children()}
    </div>
  </main>
  <Toast />
</div>
