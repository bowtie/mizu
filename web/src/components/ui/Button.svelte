<script>
  let className = '';
  export { className as class };

  export let variant = 'primary';
  export let size = 'md';
  export let block = false;
  export let href = undefined;
  export let type = 'button';

  function classes(...values) {
    return values.filter(Boolean).join(' ');
  }

  $: buttonClass = classes(
    'ui-button',
    `ui-button-${variant}`,
    `ui-button-${size}`,
    block && 'ui-button-block',
    className,
  );
</script>

{#if href}
  <a class={buttonClass} {href} {...$$restProps}>
    <slot />
  </a>
{:else}
  <button class={buttonClass} {type} {...$$restProps}>
    <slot />
  </button>
{/if}
