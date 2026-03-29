<script>
  import { Input as FoxInput, cn } from '@foxui/core';

  let className = '';
  export { className as class };

  export let value = '';
  export let type = 'text';
  export let disabled = false;
  export let invalid = false;
</script>

<div class={cn('ui-field-shell', disabled && 'is-disabled', invalid && 'is-invalid')}>
  <FoxInput
    bind:value
    {type}
    {disabled}
    class={cn('ui-field-control ui-input-control', className)}
    {...$$restProps}
  />
</div>
