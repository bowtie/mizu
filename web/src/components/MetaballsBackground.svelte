<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    metaballsFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    getShaderNoiseTexture,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Metaballs" rendered as a fixed full-screen background.
   * Liquid ocean-blue blobs merging like water droplets (based on the
   * Background preset). https://shaders.paper.design/metaballs
   */
  let {
    colorBack = '#030e1a',
    colors = ['#339df6', '#03738c', '#a3d3ff'],
    count = 13,
    size = 0.81,
    scale = 4,
    rotation = 0,
    offsetX = -0.3,
    offsetY = 0,
    speed = 0.4,
    frame = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.5,
  }: {
    colorBack?: string;
    colors?: string[];
    count?: number;
    size?: number;
    scale?: number;
    rotation?: number;
    offsetX?: number;
    offsetY?: number;
    speed?: number;
    frame?: number;
    dim?: number;
  } = $props();

  let container: HTMLDivElement;
  let mount: ShaderMount | undefined;

  onMount(() => {
    const uniforms: ShaderMountUniforms = {
      u_colorBack: getShaderColorFromString(colorBack),
      u_colors: colors.map((c) => getShaderColorFromString(c)),
      u_colorsCount: colors.length,
      u_size: size,
      u_count: count,
      u_noiseTexture: getShaderNoiseTexture(),
      u_scale: scale,
      u_rotation: rotation,
      u_fit: ShaderFitOptions.cover,
      u_offsetX: offsetX,
      u_offsetY: offsetY,
      u_originX: 0.5,
      u_originY: 0.5,
      u_worldWidth: 0,
      u_worldHeight: 0,
    };

    mount = new ShaderMount(container, metaballsFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="metaballs-bg" aria-hidden="true">
  <div bind:this={container} class="metaballs-bg__canvas"></div>
  <div class="metaballs-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .metaballs-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .metaballs-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .metaballs-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
