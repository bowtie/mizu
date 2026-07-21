<script>
  /**
   * Soft ambient light behind the glass system — a few large, heavily
   * blurred color fields drifting slowly at low opacity. Retuned to the
   * Mizu Arc palette: deep indigo base, aqua/teal light. This is the
   * only ambient motion in the app; it must stay subtle.
   */
  let { dim = 0.25 } = $props();
</script>

<div class="ambient-bg" aria-hidden="true">
  <div class="ambient-bg__blob ambient-bg__blob--1"></div>
  <div class="ambient-bg__blob ambient-bg__blob--2"></div>
  <div class="ambient-bg__blob ambient-bg__blob--3"></div>
  <div class="ambient-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .ambient-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
    background: var(--background);
  }
  .ambient-bg__blob {
    position: absolute;
    border-radius: 50%;
    filter: blur(110px);
    will-change: transform;
  }
  .ambient-bg__blob--1 {
    width: 55vw;
    height: 55vw;
    top: -18%;
    left: -12%;
    background: oklch(0.5 0.13 250);
    opacity: 0.16;
    animation: drift-1 52s ease-in-out infinite;
  }
  .ambient-bg__blob--2 {
    width: 48vw;
    height: 48vw;
    bottom: -20%;
    right: -12%;
    background: oklch(0.42 0.12 255);
    opacity: 0.15;
    animation: drift-2 64s ease-in-out infinite;
  }
  .ambient-bg__blob--3 {
    width: 38vw;
    height: 38vw;
    top: 42%;
    left: 52%;
    background: oklch(0.46 0.12 245);
    opacity: 0.08;
    animation: drift-3 76s ease-in-out infinite;
  }
  .ambient-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }

  @keyframes drift-1 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(10vw, 8vh) scale(1.08); }
    66% { transform: translate(4vw, 14vh) scale(0.94); }
  }
  @keyframes drift-2 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(-8vw, -10vh) scale(0.96); }
    66% { transform: translate(-14vw, -4vh) scale(1.1); }
  }
  @keyframes drift-3 {
    0%, 100% { transform: translate(-50%, -50%) scale(1); }
    50% { transform: translate(-38%, -58%) scale(1.14); }
  }

  @media (prefers-reduced-motion: reduce) {
    .ambient-bg__blob {
      animation: none;
    }
  }
</style>
