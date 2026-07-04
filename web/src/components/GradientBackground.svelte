<script>
  /**
   * Simple CSS gradient background (WebGradients - 123 Night Sky).
   * https://webgradients.com/gradient/123-night-sky
   */
  let { dim = 0.3 } = $props();
</script>

<div class="grad-bg" aria-hidden="true">
  <div class="grad-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .grad-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    background: linear-gradient(135deg, #1e3c72 0%, #2a5298 100%);
  }
  .grad-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
