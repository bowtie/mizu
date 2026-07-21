<script lang="ts">
	import { Toaster as Sonner, type ToasterProps as SonnerProps } from "svelte-sonner";
	import type { Component } from "svelte";
	import { mode } from "mode-watcher";
	import Loader2Icon from '@lucide/svelte/icons/loader-2';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleXIcon from '@lucide/svelte/icons/circle-x';
	import InfoIcon from '@lucide/svelte/icons/info';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

	let { ...restProps }: SonnerProps = $props();

	const SonnerComponent = Sonner as unknown as Component<Record<string, unknown>>;
</script>

<SonnerComponent
	theme={mode.current}
	class="toaster group"
	style="--normal-bg: var(--glass-overlay-bg); --normal-text: var(--color-popover-foreground); --normal-border: var(--glass-border); --border-radius: var(--radius-2xl);"
	toastOptions={{
		class: 'glass-overlay !rounded-2xl',
		success: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-success) 10%, var(--glass-overlay-bg))',
				'--normal-text': 'var(--color-success)',
				'--normal-border': 'color-mix(in oklch, var(--color-success) 28%, var(--glass-border))',
			},
		},
		error: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-destructive) 10%, var(--glass-overlay-bg))',
				'--normal-text': 'var(--color-destructive)',
				'--normal-border': 'color-mix(in oklch, var(--color-destructive) 28%, var(--glass-border))',
			},
		},
		warning: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-warning) 10%, var(--glass-overlay-bg))',
				'--normal-text': 'var(--color-warning)',
				'--normal-border': 'color-mix(in oklch, var(--color-warning) 28%, var(--glass-border))',
			},
		},
		info: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-info) 10%, var(--glass-overlay-bg))',
				'--normal-text': 'var(--color-info)',
				'--normal-border': 'color-mix(in oklch, var(--color-info) 28%, var(--glass-border))',
			},
		},
	}}
	{...restProps}
>
	{#snippet loadingIcon()}
		<Loader2Icon class="size-4 animate-spin" />
	{/snippet}
	{#snippet successIcon()}
		<CircleCheckIcon class="size-4" />
	{/snippet}
	{#snippet errorIcon()}
		<CircleXIcon class="size-4" />
	{/snippet}
	{#snippet infoIcon()}
		<InfoIcon class="size-4" />
	{/snippet}
	{#snippet warningIcon()}
		<TriangleAlertIcon class="size-4" />
	{/snippet}
</SonnerComponent>
