<script lang="ts">
	import { Select as SelectPrimitive } from "bits-ui";
	import { cn, type WithoutChild } from "$lib/utils.ts";
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';

	let {
		ref = $bindable(null),
		class: className,
		children,
		size = "default",
		...restProps
	}: WithoutChild<SelectPrimitive.TriggerProps> & {
		size?: "sm" | "default";
	} = $props();
</script>

<SelectPrimitive.Trigger
	bind:ref
	data-slot="select-trigger"
	data-size={size}
	class={cn(
		"glass-inset data-placeholder:text-muted-foreground/70 hover:border-input focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring/40 aria-invalid:border-destructive gap-1.5 rounded-xl border bg-transparent py-2.5 pr-3 pl-3.5 text-base md:text-sm transition-colors select-none data-[size=default]:h-10 data-[size=sm]:h-9 data-[size=sm]:rounded-[min(var(--radius-md),10px)] *:data-[slot=select-value]:flex *:data-[slot=select-value]:gap-1.5 [&_svg:not([class*='size-'])]:size-4 flex w-fit items-center justify-between whitespace-nowrap outline-none disabled:cursor-not-allowed disabled:opacity-50 *:data-[slot=select-value]:line-clamp-1 *:data-[slot=select-value]:flex *:data-[slot=select-value]:items-center [&_svg]:pointer-events-none [&_svg]:shrink-0",
		className
	)}
	{...restProps}
>
	{@render children?.()}
	<ChevronDownIcon class="text-muted-foreground size-4 pointer-events-none" />
</SelectPrimitive.Trigger>
