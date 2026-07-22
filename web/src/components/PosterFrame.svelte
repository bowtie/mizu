<script lang="ts">
  import { createPosterMonogram, POSTER_LOADING, POSTER_LOADED, POSTER_FAILED } from '../lib/poster-state.ts';

  let {
    src,
    alt,
    title,
  }: {
    src: string;
    alt: string;
    title: string;
  } = $props();

  let imgEl = $state<HTMLImageElement | null>(null);
  let loaded = $state(false);
  let failed = $state(false);

  // Reset when src changes
  $effect(() => {
    void src;
    loaded = false;
    failed = false;
  });

  function onLoad() {
    if (imgEl && imgEl.naturalWidth > 0) {
      loaded = true;
    } else {
      failed = true;
    }
  }

  function onError() {
    failed = true;
  }
</script>

<div class="relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-[5px] border border-surface-700 bg-surface-800 sm:w-11">
  {#if src && !failed}
    {#if !loaded}
      <div class="absolute inset-0 bg-gradient-to-b from-surface-700 to-surface-800" aria-hidden="true"></div>
    {/if}
    <img
      bind:this={imgEl}
      {src}
      {alt}
      width="160"
      height="240"
      loading="lazy"
      decoding="async"
      onload={onLoad}
      onerror={onError}
      class="h-full w-full object-cover transition-all duration-500 ease-in-out {loaded ? 'opacity-100' : 'opacity-0'}"
    />
  {:else}
    <div class="absolute inset-0 flex items-center justify-center bg-surface-800 text-sm font-semibold tracking-tight text-surface-400">
      {createPosterMonogram(title)}
    </div>
  {/if}
</div>
