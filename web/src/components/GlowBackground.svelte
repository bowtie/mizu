<script>
  /**
   * Mizu clear water background — luminous aqua gradients suggesting a bright
   * shallow pool lit from above. Three layered radials: surface shimmer,
   * mid-water glow, and a deeper pool at the bottom edge.
   */
  let { dim = 0.3 } = $props();
</script>

<div class="glow-bg" aria-hidden="true">
  <div class="glow-bg__surface"></div>
  <div class="glow-bg__mid"></div>
  <div class="glow-bg__depth"></div>
  <div class="glow-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .glow-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
    background: var(--background);
  }
  /* Bright surface shimmer from top-left, like sunlight hitting shallow water */
  .glow-bg__surface {
    position: absolute;
    inset: 0;
    background: radial-gradient(
      ellipse 100% 60% at 25% -10%,
      oklch(0.88 0.08 195) 0%,
      oklch(0.92 0.05 200) 30%,
      transparent 60%
    );
    opacity: 0.6;
  }
  /* Mid-water aqua glow, the body of the pool */
  .glow-bg__mid {
    position: absolute;
    inset: 0;
    background: radial-gradient(
      ellipse 80% 70% at 70% 40%,
      oklch(0.85 0.06 200) 0%,
      transparent 55%
    );
    opacity: 0.35;
  }
  /* Deeper water at bottom-right, cooler and more saturated */
  .glow-bg__depth {
    position: absolute;
    inset: 0;
    background: radial-gradient(
      ellipse 75% 50% at 85% 110%,
      oklch(0.72 0.10 205) 0%,
      transparent 50%
    );
    opacity: 0.35;
  }
  .glow-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
