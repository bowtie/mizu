<script lang="ts">
  import type { HTMLAttributes } from "svelte/elements";
  import { cn } from "../../lib/utils.ts";

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> = $props();
</script>

<div class={cn("rounded-xl border border-border bg-card/80 shadow-[0_2px_12px_rgba(0,0,0,0.3),0_1px_3px_rgba(0,0,0,0.2)] backdrop-blur-md", className)} {...restProps}>
  {@render children?.()}
</div>
