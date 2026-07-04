<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    warpFragmentShader,
    WarpPatterns,
    ShaderFitOptions,
    getShaderColorFromString,
    getShaderNoiseTexture,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Warp" rendered as a fixed full-screen background.
   * Uses the app's ocean-blue palette (primary → secondary → background)
   * with the Kelp preset's stripe/swirl motion. https://shaders.paper.design/warp
   */
  let {
    colors = ['#339df6', '#10293b', '#030e1a'],
    proportion = 0.67,
    softness = 0,
    distortion = 0,
    swirl = 0.2,
    swirlIterations = 3,
    shapeScale = 1,
    shape = 'stripes',
    scale = 0.8,
    rotation = 50,
    speed = 20,
    frame = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.55,
  }: {
    colors?: string[];
    proportion?: number;
    softness?: number;
    distortion?: number;
    swirl?: number;
    swirlIterations?: number;
    shapeScale?: number;
    shape?: 'checks' | 'stripes' | 'edge';
    scale?: number;
    rotation?: number;
    speed?: number;
    frame?: number;
    dim?: number;
  } = $props();

  let container: HTMLDivElement;
  let mount: ShaderMount | undefined;

  onMount(() => {
    const uniforms: ShaderMountUniforms = {
      u_colors: colors.map((c) => getShaderColorFromString(c)),
      u_colorsCount: colors.length,
      u_proportion: proportion,
      u_softness: softness,
      u_distortion: distortion,
      u_swirl: swirl,
      u_swirlIterations: swirlIterations,
      u_shapeScale: shapeScale,
      u_shape: WarpPatterns[shape],
      u_noiseTexture: getShaderNoiseTexture(),
      u_scale: scale,
      u_rotation: rotation,
      u_fit: ShaderFitOptions.none,
      u_offsetX: 0,
      u_offsetY: 0,
      u_originX: 0.5,
      u_originY: 0.5,
      u_worldWidth: 0,
      u_worldHeight: 0,
    };

    mount = new ShaderMount(container, warpFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="warp-bg" aria-hidden="true">
  <div bind:this={container} class="warp-bg__canvas"></div>
  <div class="warp-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .warp-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  /* ShaderMount positions its canvas absolutely to fill the parent. */
  .warp-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .warp-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
