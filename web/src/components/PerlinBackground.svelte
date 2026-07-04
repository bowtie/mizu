<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    perlinNoiseFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Perlin Noise" rendered as a fixed full-screen background.
   * Default preset. https://shaders.paper.design/perlin-noise
   */
  let {
    colorBack = '#632ad5',
    colorFront = '#fccff7',
    proportion = 0.35,
    softness = 0.1,
    octaveCount = 1,
    persistence = 1,
    lacunarity = 1.5,
    scale = 1,
    rotation = 0,
    speed = 0.5,
    frame = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.55,
  }: {
    colorBack?: string;
    colorFront?: string;
    proportion?: number;
    softness?: number;
    octaveCount?: number;
    persistence?: number;
    lacunarity?: number;
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
      u_colorBack: getShaderColorFromString(colorBack),
      u_colorFront: getShaderColorFromString(colorFront),
      u_proportion: proportion,
      u_softness: softness,
      u_octaveCount: octaveCount,
      u_persistence: persistence,
      u_lacunarity: lacunarity,
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

    mount = new ShaderMount(container, perlinNoiseFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="perlin-bg" aria-hidden="true">
  <div bind:this={container} class="perlin-bg__canvas"></div>
  <div class="perlin-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .perlin-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .perlin-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .perlin-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
