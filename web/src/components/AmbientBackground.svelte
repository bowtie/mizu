<script>
  /**
   * Cinema-style ambient light blobs — 2-3 large, blurred, slow-moving
   * colored circles at low opacity. Adds atmosphere without competing
   * with content. Recommended by the Cinema Mobile style guide.
   */
  let { dim = 0.3 } = $props();
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
    filter: blur(80px);
    will-change: transform;
  }
  .ambient-bg__blob--1 {
    width: 50vw;
    height: 50vw;
    top: -10%;
    left: -10%;
    background: #3b82f6;
    opacity: 0.15;
    animation: drift-1 25s ease-in-out infinite;
  }
  .ambient-bg__blob--2 {
    width: 45vw;
    height: 45vw;
    bottom: -15%;
    right: -10%;
    background: #1e3a5f;
    opacity: 0.2;
    animation: drift-2 30s ease-in-out infinite;
  }
  .ambient-bg__blob--3 {
    width: 35vw;
    height: 35vw;
    top: 40%;
    left: 50%;
    background: #2563eb;
    opacity: 0.08;
    animation: drift-3 35s ease-in-out infinite;
  }
  .ambient-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }

  @keyframes drift-1 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(15vw, 10vh) scale(1.1); }
    66% { transform: translate(5vw, 20vh) scale(0.9); }
  }
  @keyframes drift-2 {
    0%, 100% { transform: translate(0, 0) scale(1); }
    33% { transform: translate(-10vw, -15vh) scale(0.95); }
    66% { transform: translate(-20vw, -5vh) scale(1.15); }
  }
  @keyframes drift-3 {
    0%, 100% { transform: translate(-50%, -50%) scale(1); }
    50% { transform: translate(-30%, -60%) scale(1.2); }
  }

  @media (prefers-reduced-motion: reduce) {
    .ambient-bg__blob {
      animation: none;
    }
  }
</style>
