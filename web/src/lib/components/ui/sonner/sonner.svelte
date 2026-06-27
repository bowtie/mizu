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
	{...restProps}
>
	{#snippet loadingIcon()}
		<Loader2Icon class="size-4 animate-spin text-muted-foreground" />
	{/snippet}
	{#snippet successIcon()}
		<CircleCheckIcon class="size-4 text-success" />
	{/snippet}
	{#snippet errorIcon()}
		<CircleXIcon class="size-4 text-destructive" />
	{/snippet}
	{#snippet infoIcon()}
		<InfoIcon class="size-4 text-info" />
	{/snippet}
	{#snippet warningIcon()}
		<TriangleAlertIcon class="size-4 text-warning" />
	{/snippet}
</Sonner>

<style>
	:global([data-sonner-toast][data-type='success']) {
		border-left: 3px solid var(--color-success) !important;
		background: color-mix(in oklch, var(--color-success) 8%, var(--color-popover)) !important;
	}
	:global([data-sonner-toast][data-type='error']) {
		border-left: 3px solid var(--color-destructive) !important;
		background: color-mix(in oklch, var(--color-destructive) 8%, var(--color-popover)) !important;
	}
	:global([data-sonner-toast][data-type='warning']) {
		border-left: 3px solid var(--color-warning) !important;
		background: color-mix(in oklch, var(--color-warning) 8%, var(--color-popover)) !important;
	}
	:global([data-sonner-toast][data-type='info']) {
		border-left: 3px solid var(--color-info) !important;
		background: color-mix(in oklch, var(--color-info) 8%, var(--color-popover)) !important;
	}
</style>
