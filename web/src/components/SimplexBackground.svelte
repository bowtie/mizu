<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    simplexNoiseFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Simplex Noise" rendered as a fixed full-screen background.
   * Bubblegum preset tuned for the dark navy slate theme.
   * coolors.co/palette/0466c8-0353a4-023e7d-002855-001845-001233-33415c-5c677d-7d8597-979dac
   * https://shaders.paper.design/simplex-noise
   */
  let {
    colors = ['#001233', '#001845', '#002855', '#023e7d', '#0466c8'],
    stepsPerColor = 1,
    softness = 1,
    scale = 1.6,
    rotation = 0,
    speed = 1.2,
    frame = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.55,
  }: {
    colors?: string[];
    stepsPerColor?: number;
    softness?: number;
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
      u_stepsPerColor: stepsPerColor,
      u_softness: softness,
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

    mount = new ShaderMount(container, simplexNoiseFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="simplex-bg" aria-hidden="true">
  <div bind:this={container} class="simplex-bg__canvas"></div>
  <div class="simplex-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .simplex-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .simplex-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .simplex-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
