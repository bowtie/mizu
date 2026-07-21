<script lang="ts">
  import { onMount } from 'svelte';
  import { gsap } from 'gsap';
  import type { Snippet } from 'svelte';

  /**
   * Spotlight grid wrapper adapted from Svelte Bits ChromaGrid.
   * Wraps existing grid content with a mouse-following spotlight that
   * desaturates content outside the radius, revealing color near the cursor.
   * https://sveltebits.com/components/chroma-grid
   */
  type Props = {
    children: Snippet;
    radius?: number;
    damping?: number;
    fadeOut?: number;
    ease?: string;
    class?: string;
  };

  let {
    children,
    radius = 300,
    damping = 0.45,
    fadeOut = 0.6,
    ease = 'power3.out',
    class: className = '',
  }: Props = $props();

  let rootRef: HTMLDivElement;
  let fadeRef: HTMLDivElement;
  let setX: ((v: number | string) => void) | null = null;
  let setY: ((v: number | string) => void) | null = null;
  const pos = { x: 0, y: 0 };

  onMount(() => {
    setX = gsap.quickSetter(rootRef, '--x', 'px') as (v: number | string) => void;
    setY = gsap.quickSetter(rootRef, '--y', 'px') as (v: number | string) => void;
    const { width, height } = rootRef.getBoundingClientRect();
    pos.x = width / 2;
    pos.y = height / 2;
    setX(pos.x);
    setY(pos.y);
  });

  function moveTo(x: number, y: number) {
    gsap.to(pos, {
      x,
      y,
      duration: damping,
      ease,
      onUpdate: () => {
        setX?.(pos.x);
        setY?.(pos.y);
      },
      overwrite: true,
    });
  }
  function handleMove(e: PointerEvent) {
    const r = rootRef.getBoundingClientRect();
    moveTo(e.clientX - r.left, e.clientY - r.top);
    gsap.to(fadeRef, { opacity: 0, duration: 0.25, overwrite: true });
  }
  function handleLeave() {
    gsap.to(fadeRef, { opacity: 1, duration: fadeOut, overwrite: true });
  }
</script>

<div
  bind:this={rootRef}
  onpointermove={handleMove}
  onpointerleave={handleLeave}
  class="spotlight-grid {className}"
  style="--r:{radius}px;"
  role="presentation"
>
  {@render children?.()}
  <div class="spotlight-overlay"></div>
  <div bind:this={fadeRef} class="spotlight-fade"></div>
</div>

<style>
  .spotlight-grid {
    position: relative;
    --x: 50%;
    --y: 50%;
    --r: 220px;
  }
  .spotlight-overlay {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 3;
    backdrop-filter: grayscale(1) brightness(0.78);
    -webkit-backdrop-filter: grayscale(1) brightness(0.78);
    background: rgba(0, 0, 0, 0.001);
    mask-image: radial-gradient(
      circle var(--r) at var(--x) var(--y),
      transparent 0%,
      transparent 15%,
      rgba(0, 0, 0, 0.1) 30%,
      rgba(0, 0, 0, 0.22) 45%,
      rgba(0, 0, 0, 0.35) 60%,
      rgba(0, 0, 0, 0.5) 75%,
      rgba(0, 0, 0, 0.68) 88%,
      white 100%
    );
    -webkit-mask-image: radial-gradient(
      circle var(--r) at var(--x) var(--y),
      transparent 0%,
      transparent 15%,
      rgba(0, 0, 0, 0.1) 30%,
      rgba(0, 0, 0, 0.22) 45%,
      rgba(0, 0, 0, 0.35) 60%,
      rgba(0, 0, 0, 0.5) 75%,
      rgba(0, 0, 0, 0.68) 88%,
      white 100%
    );
  }
  .spotlight-fade {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 4;
    backdrop-filter: grayscale(1) brightness(0.78);
    -webkit-backdrop-filter: grayscale(1) brightness(0.78);
    background: rgba(0, 0, 0, 0.001);
    mask-image: radial-gradient(
      circle var(--r) at var(--x) var(--y),
      white 0%,
      white 15%,
      rgba(255, 255, 255, 0.9) 30%,
      rgba(255, 255, 255, 0.78) 45%,
      rgba(255, 255, 255, 0.65) 60%,
      rgba(255, 255, 255, 0.5) 75%,
      rgba(255, 255, 255, 0.32) 88%,
      transparent 100%
    );
    -webkit-mask-image: radial-gradient(
      circle var(--r) at var(--x) var(--y),
      white 0%,
      white 15%,
      rgba(255, 255, 255, 0.9) 30%,
      rgba(255, 255, 255, 0.78) 45%,
      rgba(255, 255, 255, 0.65) 60%,
      rgba(255, 255, 255, 0.5) 75%,
      rgba(255, 255, 255, 0.32) 88%,
      transparent 100%
    );
    opacity: 1;
    transition: opacity 0.25s ease;
  }
</style>
