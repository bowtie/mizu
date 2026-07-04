<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    staticMeshGradientFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Static Mesh Gradient" rendered as a fixed full-screen
   * background. Defaults to the Sea preset.
   * https://shaders.paper.design/static-mesh-gradient
   */
  let {
    colors = ['#3b82f6', '#1e3a5f', '#0a0f1e'],
    positions = 0,
    waveX = 0.53,
    waveXShift = 0,
    waveY = 0.95,
    waveYShift = 0.64,
    mixing = 0.5,
    grainMixer = 0,
    grainOverlay = 0,
    scale = 1,
    rotation = 0,
    speed = 0,
    frame = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.4,
  }: {
    colors?: string[];
    positions?: number;
    waveX?: number;
    waveXShift?: number;
    waveY?: number;
    waveYShift?: number;
    mixing?: number;
    grainMixer?: number;
    grainOverlay?: number;
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
      u_positions: positions,
      u_waveX: waveX,
      u_waveXShift: waveXShift,
      u_waveY: waveY,
      u_waveYShift: waveYShift,
      u_mixing: mixing,
      u_grainMixer: grainMixer,
      u_grainOverlay: grainOverlay,
      u_scale: scale,
      u_rotation: rotation,
      u_fit: ShaderFitOptions.contain,
      u_offsetX: 0,
      u_offsetY: 0,
      u_originX: 0.5,
      u_originY: 0.5,
      u_worldWidth: 0,
      u_worldHeight: 0,
    };

    mount = new ShaderMount(container, staticMeshGradientFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="mesh-bg" aria-hidden="true">
  <div bind:this={container} class="mesh-bg__canvas"></div>
  <div class="mesh-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .mesh-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .mesh-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .mesh-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
