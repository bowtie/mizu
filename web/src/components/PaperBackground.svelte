<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ShaderMount,
    paperTextureFragmentShader,
    ShaderFitOptions,
    getShaderColorFromString,
    getShaderNoiseTexture,
    type ShaderMountUniforms,
  } from '@paper-design/shaders';

  /**
   * Paper Shaders "Paper Texture" rendered as a fixed full-screen background.
   * Default preset with dark blue colors.
   * https://shaders.paper.design/paper-texture
   */
  let {
    colorFront = '#00c6fb',
    colorBack = '#005bea',
    contrast = 0.3,
    roughness = 0.4,
    fiber = 0.3,
    fiberSize = 0.2,
    crumples = 0.3,
    crumpleSize = 0.35,
    folds = 0.65,
    foldCount = 5,
    fade = 0,
    drops = 0.2,
    seed = 5.8,
    scale = 0.6,
    rotation = 0,
    speed = 0,
    frame = 0,
    dim = 0.55,
  }: {
    colorFront?: string;
    colorBack?: string;
    contrast?: number;
    roughness?: number;
    fiber?: number;
    fiberSize?: number;
    crumples?: number;
    crumpleSize?: number;
    folds?: number;
    foldCount?: number;
    fade?: number;
    drops?: number;
    seed?: number;
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
      u_image: '',
      u_colorFront: getShaderColorFromString(colorFront),
      u_colorBack: getShaderColorFromString(colorBack),
      u_contrast: contrast,
      u_roughness: roughness,
      u_fiber: fiber,
      u_fiberSize: fiberSize,
      u_crumples: crumples,
      u_crumpleSize: crumpleSize,
      u_foldCount: foldCount,
      u_folds: folds,
      u_fade: fade,
      u_drops: drops,
      u_seed: seed,
      ...getShaderNoiseTexture(),
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

    mount = new ShaderMount(container, paperTextureFragmentShader, uniforms, undefined, speed, frame);

    return () => {
      mount?.dispose();
      mount = undefined;
    };
  });
</script>

<div class="paper-bg" aria-hidden="true">
  <div bind:this={container} class="paper-bg__canvas"></div>
  <div class="paper-bg__dim" style="opacity: {dim}"></div>
</div>

<style>
  .paper-bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    overflow: hidden;
  }
  .paper-bg__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }
  .paper-bg__dim {
    position: absolute;
    inset: 0;
    background: var(--background);
    transition: opacity 200ms ease;
  }
</style>
