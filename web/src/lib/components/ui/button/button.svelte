<script lang="ts" module>
	import { cn, type WithElementRef } from "$lib/utils.ts";
	import type { HTMLAnchorAttributes, HTMLButtonAttributes } from "svelte/elements";
	import { type VariantProps, tv } from "tailwind-variants";

	export const buttonVariants = tv({
		base: "focus-visible:border-ring aria-invalid:border-destructive dark:aria-invalid:border-destructive/50 rounded-xl border border-transparent bg-clip-padding text-sm font-medium active:not-aria-[haspopup]:translate-y-px [&_svg:not([class*='size-'])]:size-4 group/button inline-flex shrink-0 items-center justify-center whitespace-nowrap transition-all outline-none select-none cursor-pointer disabled:pointer-events-none disabled:opacity-50 disabled:cursor-not-allowed [&_svg]:pointer-events-none [&_svg]:shrink-0",
		variants: {
			variant: {
				default: "border-transparent bg-primary font-semibold text-primary-foreground [a]:hover:bg-primary/85",
				outline: "border-border bg-transparent hover:bg-accent hover:text-foreground aria-expanded:bg-accent aria-expanded:text-foreground",
				secondary: "border-border bg-surface-1 text-secondary-foreground hover:border-input hover:bg-accent aria-expanded:bg-accent",
				ghost: "hover:bg-accent hover:text-foreground aria-expanded:bg-accent aria-expanded:text-foreground",
				destructive: "border-destructive/40 bg-destructive/15 text-destructive hover:bg-destructive/25 focus-visible:border-destructive/40 focus-visible:ring-destructive/20",
				link: "text-primary underline-offset-4 hover:underline",
			},
			size: {
				default: "h-10 gap-2 px-4 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
				xs: "h-7 gap-1 rounded-[min(var(--radius-md),10px)] px-2.5 text-xs in-data-[slot=button-group]:rounded-xl has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
				sm: "h-9 gap-1.5 rounded-[min(var(--radius-md),12px)] px-3 text-[0.8rem] in-data-[slot=button-group]:rounded-xl has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 [&_svg:not([class*='size-'])]:size-3.5",
				lg: "h-11 gap-2 px-6 has-data-[icon=inline-end]:pr-4 has-data-[icon=inline-start]:pl-4",
				icon: "size-10",
				"icon-xs": "size-7 rounded-[min(var(--radius-md),10px)] in-data-[slot=button-group]:rounded-xl [&_svg:not([class*='size-'])]:size-3",
				"icon-sm": "size-9 rounded-[min(var(--radius-md),12px)] in-data-[slot=button-group]:rounded-xl",
				"icon-lg": "size-11",
			},
		},
		defaultVariants: {
			variant: "default",
			size: "default",
		},
	});

	export type ButtonVariant = VariantProps<typeof buttonVariants>["variant"];
	export type ButtonSize = VariantProps<typeof buttonVariants>["size"];

	export type ButtonProps = WithElementRef<HTMLButtonAttributes> &
		WithElementRef<HTMLAnchorAttributes> & {
			variant?: ButtonVariant;
			size?: ButtonSize;
		};
</script>

<script lang="ts">
	let {
		class: className,
		variant = "default",
		size = "default",
		ref = $bindable(null),
		href = undefined,
		type = "button",
		disabled,
		children,
		...restProps
	}: ButtonProps = $props();
</script>

{#if href}
	<a
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		href={disabled ? undefined : href}
		aria-disabled={disabled}
		role={disabled ? "link" : undefined}
		tabindex={disabled ? -1 : undefined}
		{...restProps}
	>
		{@render children?.()}
	</a>
{:else}
	<button
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		{type}
		{disabled}
		{...restProps}
	>
		{@render children?.()}
	</button>
{/if}
