<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    wavesFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Waves" rendered as a fixed full-screen background.
   * Ocean-blue waves on the app's dark navy background. Static (no animation).
   * https://shaders.paper.design/waves
   */
  let {
    colorFront = '#a3d3ff',
    colorBack = '#030e1a',
    shape = 2.25,
    frequency = 0.2,
    amplitude = 1,
    spacing = 1.25,
    proportion = 1,
    softness = 0,
    scale = 1.7,
    rotation = 0,
    /** 0-1, how strongly a dark overlay dims the shader for readability. */
    dim = 0.45,
  }: {
    colorFront?: string;
    colorBack?: string;
    shape?: number;
    frequency?: number;
    amplitude?: number;
    spacing?: number;
    proportion?: number;
    softness?: number;
    scale?: number;
    rotation?: number;
    dim?: number;
  } = $props();

  let container: HTMLDivElement;
  let mount: ShaderMount | undefined;

  onMount(() => {
    const uniforms: ShaderMountUniforms = {
      u_colorFront: getShaderColorFromString(colorFront),
      u_colorBack: getShaderColorFromString(colorBack),
      u_shape: shape,
      u_frequency: frequency,
      u_amplitude: amplitude,
      u_spacing: spacing,
      u_proportion: proportion,
      u_softness: softness,
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

    // Static shader: speed 0
    mount = new ShaderMount(container, wavesFragmentShader, uniforms, undefined, 0, 0);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="waves-bg" aria-hidden="true">
  <div bind:this={container} class="waves-bg__canvas"></div>
  <div class="waves-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .waves-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .waves-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .waves-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
