<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    godRaysFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    getShaderNoiseTexture,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "God Rays" rendered as a fixed full-screen background.
   * Underwater light shafts in the app's ocean-blue palette (based on the
   * Ether preset). https://shaders.paper.design/god-rays
   */
  let {
    colorBack = '#030e1a',
    colorBloom = '#a3d3ff',
    colors = ['#339df6a6', '#a3d3ffbe', '#08192a'],
    density = 0.08,
    spotty = 0.6,
    midIntensity = 0.5,
    midSize = 0.12,
    intensity = 0.55,
    bloom = 0.6,
    scale = 1,
    rotation = 0,
    offsetX = -0.4,
    offsetY = -0.55,
    speed = 0.5,
    frame = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.4,
  }: {
    colorBack?: string;
    colorBloom?: string;
    colors?: string[];
    density?: number;
    spotty?: number;
    midIntensity?: number;
    midSize?: number;
    intensity?: number;
    bloom?: number;
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
      u_colorBloom: getShaderColorFromString(colorBloom),
      u_colorBack: getShaderColorFromString(colorBack),
      u_colors: colors.map((c) => getShaderColorFromString(c)),
      u_colorsCount: colors.length,
      u_density: density,
      u_spotty: spotty,
      u_midIntensity: midIntensity,
      u_midSize: midSize,
      u_intensity: intensity,
      u_bloom: bloom,
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

    mount = new ShaderMount(container, godRaysFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="godrays-bg" aria-hidden="true">
  <div bind:this={container} class="godrays-bg__canvas"></div>
  <div class="godrays-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .godrays-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .godrays-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .godrays-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
