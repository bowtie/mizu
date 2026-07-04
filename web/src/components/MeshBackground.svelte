<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    meshGradientFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders animated "Mesh Gradient" rendered as a fixed full-screen
   * background. Dark navy slate palette: deep navy to bright blue flow.
   * coolors.co/palette/0466c8-0353a4-023e7d-002855-001845-001233-33415c-5c677d-7d8597-979dac
   * https://shaders.paper.design/mesh-gradient
   */
  let {
    colors = ['#001233', '#001845', '#002855', '#023e7d', '#0353a4', '#0466c8'],
    distortion = 0.8,
    swirl = 0.6,
    grainMixer = 0,
    grainOverlay = 0,
    scale = 1.3,
    rotation = 0,
    speed = 0.2,
    frame = 0,
    /** 0-1, how strongly the background overlay softens the shader for readability. */
    dim = 0.7,
  }: {
    colors?: string[];
    distortion?: number;
    swirl?: number;
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
      u_distortion: distortion,
      u_swirl: swirl,
      u_grainMixer: grainMixer,
      u_grainOverlay: grainOverlay,
      u_scale: scale,
      u_rotation: rotation,
      u_fit: ShaderFitOptions.cover,
      u_offsetX: 0,
      u_offsetY: 0,
      u_originX: 0.5,
      u_originY: 0.5,
      u_worldWidth: 0,
      u_worldHeight: 0,
    };

    mount = new ShaderMount(container, meshGradientFragmentShader, uniforms, undefined, speed, frame);

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
