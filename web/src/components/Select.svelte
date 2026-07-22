<script lang="ts">
  import { ChevronDown } from 'lucide-svelte';

  interface Option {
    value: string;
    label: string;
  }

  let {
    options,
    value = $bindable(),
    placeholder = '',
    class: className = '',
  }: {
    options: Option[];
    value?: string;
    placeholder?: string;
    class?: string;
  } = $props();

  let open = $state(false);
  let rootEl: HTMLDivElement | null = $state(null);

  const selectedLabel = $derived(options.find((o) => o.value === value)?.label ?? '');

  function toggle() {
    open = !open;
  }

  function select(val: string) {
    value = val;
    open = false;
  }

  function handleClickOutside(e: MouseEvent) {
    if (rootEl && !rootEl.contains(e.target as Node)) {
      open = false;
    }
  }

  $effect(() => {
    if (open) {
      document.addEventListener('click', handleClickOutside);
      return () => document.removeEventListener('click', handleClickOutside);
    }
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') open = false;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      toggle();
    }
  }
</script>

<div bind:this={rootEl} class="relative {className}">
  <button
    type="button"
    onclick={toggle}
    onkeydown={handleKeydown}
    class="input field-base flex h-11 w-full cursor-pointer items-center justify-between rounded-lg px-3 text-left text-xs shadow-none"
    aria-haspopup="listbox"
    aria-expanded={open}
  >
    <span class={selectedLabel ? 'text-surface-50' : 'text-surface-400'}>
      {selectedLabel || placeholder}
    </span>
    <ChevronDown size={14} class="shrink-0 text-surface-400 transition-transform {open ? 'rotate-180' : ''}" />
  </button>

  {#if open}
    <ul
      class="card absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-surface-700 bg-surface-900 p-1 shadow-xl"
      role="listbox"
    >
      {#each options as opt (opt.value)}
        <li>
          <button
            type="button"
            onclick={() => select(opt.value)}
            class="flex w-full items-center rounded-md px-3 py-2 text-left text-xs transition-colors hover:bg-surface-800 {opt.value === value ? 'bg-surface-800 text-primary-500' : 'text-surface-200'}"
            role="option"
            aria-selected={opt.value === value}
          >
            {opt.label}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
