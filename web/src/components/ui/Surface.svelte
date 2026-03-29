<script>
  let className = '';
  export { className as class };

  export let tag = 'div';
  export let variant = 'default';
  export let padding = 'md';
  export let blur = false;
</script>

<svelte:element
  this={tag}
  class={`ui-surface ui-surface-${variant} ui-surface-padding-${padding} ${blur ? 'ui-surface-blur' : ''} ${className}`.trim()}
  {...$$restProps}
>
  <slot />
</svelte:element>
