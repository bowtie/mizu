<script>
  let className = '';
  export { className as class };

  export let value = '';
  export let type = 'text';
  export let disabled = false;
  export let invalid = false;

  function classes(...values) {
    return values.filter(Boolean).join(' ');
  }
</script>

<div class={classes('ui-field-shell', disabled && 'is-disabled', invalid && 'is-invalid')}>
  <input
    bind:value
    {type}
    {disabled}
    class={classes('ui-field-control ui-input-control', className)}
    {...$$restProps}
  />
</div>
