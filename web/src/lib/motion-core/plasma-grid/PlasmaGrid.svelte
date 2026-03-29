<script lang="ts">
	import { Canvas } from "@threlte/core";
	import Scene from "./PlasmaGridScene.svelte";
	import { NoToneMapping } from "three";
	import type { HTMLAttributes } from "svelte/elements";
	import type { ComponentProps } from "svelte";

	type SceneProps = ComponentProps<typeof Scene>;

	interface Props {
		/**
		 * The base background color of the effect.
		 * @default "#111113"
		 */
		color?: SceneProps["color"];
		/**
		 * The color used for the plasma noise gradients.
		 * @default "#FF6900"
		 */
		highlightColor?: SceneProps["highlightColor"];
		/**
		 * Additional CSS classes for the container.
		 */
		class?: string;
		[key: string]: unknown;
	}

	let {
		color,
		highlightColor,
		class: className = "",
		...rest
	}: Props = $props();

	const dpr = typeof window !== "undefined" ? window.devicePixelRatio : 1;
</script>

<div class={`plasma-grid ${className}`.trim()} {...(rest as HTMLAttributes<HTMLDivElement>)}>
	<div class="plasma-grid-canvas">
		<Canvas {dpr} toneMapping={NoToneMapping}>
			<Scene {color} {highlightColor} />
		</Canvas>
	</div>
</div>

<style>
	.plasma-grid {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: hidden;
	}

	.plasma-grid-canvas {
		position: absolute;
		inset: 0;
	}
</style>
