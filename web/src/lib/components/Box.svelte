<script lang="ts">
  import type { HTMLAttributes } from "svelte/elements";
  import { cn } from "../../lib/utils.ts";

  let {
    class: className,
    children,
    ...restProps
  }: HTMLAttributes<HTMLDivElement> = $props();
</script>

<div class={cn("rounded-2xl border border-border bg-card/60", className)} {...restProps}>
  {@render children?.()}
</div>
