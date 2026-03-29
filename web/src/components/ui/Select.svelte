<script>
  import { Select as SelectPrimitive } from 'bits-ui';
  import { cn } from '@foxui/core';
  import { Check, ChevronDown } from 'lucide-svelte';

  let className = '';
  export { className as class };

  export let value = '';
  export let items = [];
  export let placeholder = 'Select an option';
  export let disabled = false;
  export let contentProps = {};

  $: selectedItem = items.find((item) => item.value === value);
</script>

<SelectPrimitive.Root bind:value {disabled} {...$$restProps}>
  <SelectPrimitive.Trigger
    class={cn('ui-field-shell ui-select-trigger', disabled && 'is-disabled', className)}
    aria-label={selectedItem?.label ?? placeholder}
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
      class={cn('ui-select-content', contentProps?.class)}
    >
      <SelectPrimitive.Viewport class="ui-select-viewport">
        {#each items as item (item.value)}
          <SelectPrimitive.Item value={item.value} label={item.label} disabled={item.disabled}>
            {#snippet children({ selected })}
              <div class={cn('ui-select-item', selected && 'is-selected', item.disabled && 'is-disabled')}>
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
