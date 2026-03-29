<script lang="ts">
	import { Canvas } from "@threlte/core";
	import { NoToneMapping } from "three";
	import type { HTMLAttributes } from "svelte/elements";
	import type { ComponentProps } from "svelte";
	import Scene from "./SpecularBandScene.svelte";

	type SceneProps = ComponentProps<typeof Scene>;

	interface Props {
		/**
		 * Additional CSS classes for the container.
		 */
		class?: string;
		/**
		 * Base color of the specular bands.
		 * @default "#FF6900"
		 */
		color?: SceneProps["color"];
		/**
		 * Color of the background.
		 * @default "#000000"
		 */
		backgroundColor?: SceneProps["backgroundColor"];
		/**
		 * Animation speed multiplier.
		 * @default 1.0
		 */
		speed?: SceneProps["speed"];
		/**
		 * Lens distortion intensity.
		 * @default 0.2
		 */
		distortion?: SceneProps["distortion"];
		/**
		 * Amount of hue shift for secondary bands (in degrees).
		 * @default 30.0
		 */
		hueShift?: SceneProps["hueShift"];
		/**
		 * Global intensity/brightness of the effect.
		 * @default 1.0
		 */
		intensity?: SceneProps["intensity"];
		[key: string]: unknown;
	}

	let {
		class: className = "",
		color = "#FF6900",
		backgroundColor = "#000000",
		speed = 1.0,
		distortion = 0.2,
		hueShift = 30.0,
		intensity = 1.0,
		...rest
	}: Props = $props();

	const dpr = typeof window !== "undefined" ? window.devicePixelRatio : 1;
</script>

<div class={`specular-band ${className}`.trim()} {...(rest as HTMLAttributes<HTMLDivElement>)}>
	<div class="specular-band-canvas">
		<Canvas {dpr} toneMapping={NoToneMapping}>
			<Scene
				{color}
				{backgroundColor}
				{speed}
				{distortion}
				{hueShift}
				{intensity}
			/>
		</Canvas>
	</div>
</div>

<style>
	.specular-band {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: hidden;
	}

	.specular-band-canvas {
		position: absolute;
		inset: 0;
	}
</style>
