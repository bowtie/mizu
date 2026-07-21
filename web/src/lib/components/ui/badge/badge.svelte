<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const badgeVariants = tv({
		base: "h-6 gap-1 rounded-full border border-transparent px-2.5 py-0.5 text-xs font-medium transition-all has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&>svg]:size-3.5! focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive group/badge inline-flex w-fit shrink-0 items-center justify-center overflow-hidden whitespace-nowrap transition-colors focus-visible:ring-[3px] [&>svg]:pointer-events-none",
		variants: {
			variant: {
				default: "border-primary/30 bg-primary/20 text-primary [a]:hover:bg-primary/30",
				secondary: "border-border bg-secondary text-secondary-foreground [a]:hover:bg-accent",
				destructive: "border-destructive/25 bg-destructive/15 text-destructive [a]:hover:bg-destructive/25 focus-visible:ring-destructive/20",
				outline: "border-border text-foreground [a]:hover:bg-muted [a]:hover:text-muted-foreground",
				ghost: "hover:bg-muted hover:text-muted-foreground",
				link: "text-primary underline-offset-4 hover:underline",
				success: "border-success/25 bg-success/15 text-success [a]:hover:bg-success/25",
				warning: "border-warning/25 bg-warning/15 text-warning [a]:hover:bg-warning/25",
				info: "border-info/25 bg-info/15 text-info [a]:hover:bg-info/25",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type BadgeVariant = VariantProps<typeof badgeVariants>["variant"];
</script>

<script lang="ts">
	import type { HTMLAnchorAttributes } from "svelte/elements";
	import { cn, type WithElementRef } from "$lib/utils.ts";

	let {
		ref = $bindable(null),
		href,
		class: className,
		variant = "default",
		children,
		...restProps
	}: WithElementRef<HTMLAnchorAttributes> & {
		variant?: BadgeVariant;
	} = $props();
</script>

<svelte:element
	this={href ? "a" : "span"}
	bind:this={ref}
	data-slot="badge"
	{href}
	class={cn(badgeVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
</svelte:element>
