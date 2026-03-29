<script>
  import { Select as FoxSelect, cn } from '@foxui/core';

  let className = '';
  export { className as class };

  export let value = '';
  export let items = [];
  export let placeholder = 'Select an option';
  export let disabled = false;
  export let contentProps = {};

  $: mergedContentProps = {
    ...contentProps,
    class: cn('ui-select-content', contentProps?.class),
  };
</script>

<div class={cn('ui-select-wrapper', disabled && 'is-disabled', className)}>
  <FoxSelect
    bind:value
    {items}
    {placeholder}
    {disabled}
    contentProps={mergedContentProps}
    {...$$restProps}
  />
</div>
