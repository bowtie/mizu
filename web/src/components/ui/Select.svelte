<script>
  import { Select as SelectPrimitive } from 'bits-ui';
  import { Check, ChevronDown } from 'lucide-svelte';

  let className = '';
  export { className as class };

  export let value = '';
  export let items = [];
  export let placeholder = 'Select an option';
  export let disabled = false;
  export let contentProps = {};

  function classes(...values) {
    return values.filter(Boolean).join(' ');
  }

  const triggerPropNames = new Set([
    'id',
    'tabindex',
    'autofocus',
    'title',
    'aria-label',
    'aria-labelledby',
    'aria-describedby',
    'aria-controls',
    'aria-expanded',
    'aria-required',
    'aria-invalid',
    'aria-errormessage',
  ]);

  $: selectedItem = items.find((item) => item.value === value);
  $: contentClass = classes('ui-select-content', contentProps?.class);
  $: ({ rootProps, triggerProps } = Object.entries($$restProps).reduce(
    (acc, [key, propValue]) => {
      if (triggerPropNames.has(key) || key.startsWith('aria-') || key.startsWith('data-')) {
        acc.triggerProps[key] = propValue;
      } else {
        acc.rootProps[key] = propValue;
      }
      return acc;
    },
    { rootProps: {}, triggerProps: {} },
  ));
</script>

<div class={classes('ui-select-wrapper', disabled && 'is-disabled', className)}>
  <SelectPrimitive.Root bind:value type="single" {disabled} {...rootProps}>
    <SelectPrimitive.Trigger
      class={classes('ui-field-shell ui-select-trigger', disabled && 'is-disabled')}
      aria-label={selectedItem?.label ?? placeholder}
      {...triggerProps}
    >
      <span class:ui-select-placeholder={!selectedItem}>
        {selectedItem?.label ?? placeholder}
      </span>
      <ChevronDown size={16} />
    </SelectPrimitive.Trigger>

    <SelectPrimitive.Portal>
      <SelectPrimitive.Content
        sideOffset={6}
        {...contentProps}
        class={contentClass}
      >
        <SelectPrimitive.Viewport class="ui-select-viewport">
          {#each items as item (item.value)}
            <SelectPrimitive.Item value={item.value} label={item.label} disabled={item.disabled}>
              {#snippet children({ selected, highlighted })}
                <div class={classes('ui-select-item', selected && 'is-selected', highlighted && 'is-highlighted', item.disabled && 'is-disabled')}>
                  <span>{item.label}</span>
                  {#if selected}
                    <Check size={16} />
                  {/if}
                </div>
              {/snippet}
            </SelectPrimitive.Item>
          {/each}
        </SelectPrimitive.Viewport>
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  </SelectPrimitive.Root>
</div>
