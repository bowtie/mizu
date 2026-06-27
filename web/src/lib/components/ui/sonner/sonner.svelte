<script lang="ts">
	import { Toaster as Sonner, type ToasterProps as SonnerProps } from "svelte-sonner";
	import { mode } from "mode-watcher";
	import Loader2Icon from '@lucide/svelte/icons/loader-2';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleXIcon from '@lucide/svelte/icons/circle-x';
	import InfoIcon from '@lucide/svelte/icons/info';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

	let { ...restProps }: SonnerProps = $props();
</script>

<Sonner
	theme={mode.current}
	class="toaster group"
	style="--normal-bg: var(--color-popover); --normal-text: var(--color-popover-foreground); --normal-border: var(--color-border);"
	toastOptions={{
		success: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-success) 12%, var(--color-popover))',
				'--normal-text': 'var(--color-success)',
				'--normal-border': 'color-mix(in oklch, var(--color-success) 30%, var(--color-border))',
			},
		},
		error: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-destructive) 12%, var(--color-popover))',
				'--normal-text': 'var(--color-destructive)',
				'--normal-border': 'color-mix(in oklch, var(--color-destructive) 30%, var(--color-border))',
			},
		},
		warning: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-warning) 12%, var(--color-popover))',
				'--normal-text': 'var(--color-warning)',
				'--normal-border': 'color-mix(in oklch, var(--color-warning) 30%, var(--color-border))',
			},
		},
		info: {
			style: {
				'--normal-bg': 'color-mix(in oklch, var(--color-info) 12%, var(--color-popover))',
				'--normal-text': 'var(--color-info)',
				'--normal-border': 'color-mix(in oklch, var(--color-info) 30%, var(--color-border))',
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
</Sonner>
