<script>
  import { createEventDispatcher } from 'svelte';
  import { ChevronDown, ChevronUp } from 'lucide-svelte';

  export let id = undefined;
  export let name = undefined;
  export let value = 0;
  export let min = 0;
  export let max = Number.POSITIVE_INFINITY;
  export let step = 1;
  export let disabled = false;
  export let ariaLabel = 'Number input';

  const dispatch = createEventDispatcher();

  let draftValue = String(value ?? min ?? 0);
  let isFocused = false;

  const clamp = (next) => {
    const parsed = Number(next);
    if (!Number.isFinite(parsed)) return min;
    return Math.min(max, Math.max(min, parsed));
  };

  $: if (!isFocused) {
    draftValue = String(clamp(value));
  }

  function sync(nextValue, emitChange = false) {
    const normalized = clamp(nextValue);
    value = normalized;
    draftValue = String(normalized);
    dispatch('input', { value: normalized });
    if (emitChange) {
      dispatch('change', { value: normalized });
    }
  }

  function handleInput(event) {
    const cleaned = event.currentTarget.value.replace(/[^\d-]/g, '');
    draftValue = cleaned;

    if (cleaned === '' || cleaned === '-') {
      value = min;
      dispatch('input', { value });
      return;
    }

    const parsed = Number(cleaned);
    if (Number.isFinite(parsed)) {
      value = clamp(parsed);
      dispatch('input', { value });
    }
  }

  function handleBlur() {
    isFocused = false;
    sync(draftValue === '' || draftValue === '-' ? min : draftValue, true);
  }

  function handleFocus() {
    isFocused = true;
  }

  function adjust(direction) {
    if (disabled) return;
    const baseValue = draftValue === '' || draftValue === '-' ? value : Number(draftValue);
    sync((Number.isFinite(baseValue) ? baseValue : value) + direction * step, true);
  }

  function handleKeydown(event) {
    if (disabled) return;

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      adjust(1);
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      adjust(-1);
    }
  }
</script>

<div class="number-input" class:is-disabled={disabled}>
  <input
    {id}
    {name}
    type="text"
    inputmode="numeric"
    pattern="[0-9]*"
    class="number-input-field"
    bind:value={draftValue}
    aria-label={ariaLabel}
    {disabled}
    on:input={handleInput}
    on:focus={handleFocus}
    on:blur={handleBlur}
    on:keydown={handleKeydown}
  />
  <div class="number-input-stepper" aria-hidden="true">
    <button type="button" class="number-step-button" on:click={() => adjust(1)} disabled={disabled || value >= max} tabindex="-1">
      <ChevronUp size={14} />
    </button>
    <button type="button" class="number-step-button" on:click={() => adjust(-1)} disabled={disabled || value <= min} tabindex="-1">
      <ChevronDown size={14} />
    </button>
  </div>
</div>

<style>
  .number-input {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 32px;
    min-height: 40px;
    width: 100%;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--surface-elevated) 78%, rgba(7, 16, 29, 0.96) 22%);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
    overflow: hidden;
    transition: box-shadow var(--transition), background var(--transition), opacity var(--transition);
  }

  .number-input:focus-within {
    box-shadow: 0 0 0 2px var(--focus-ring-soft);
  }

  .number-input.is-disabled {
    background: color-mix(in srgb, var(--surface-elevated) 54%, rgba(4, 10, 18, 0.99) 46%);
    opacity: 0.72;
  }

  .number-input-field {
    width: 100%;
    min-width: 0;
    border: 0;
    outline: none;
    padding: 0 13px;
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
    caret-color: var(--accent-cream);
  }

  .number-input-field:disabled {
    cursor: not-allowed;
  }

  .number-input-stepper {
    display: grid;
    grid-template-rows: 1fr 1fr;
    border-left: 1px solid color-mix(in srgb, var(--border) 76%, rgba(255, 255, 255, 0.05) 24%);
    background: color-mix(in srgb, rgba(4, 10, 18, 0.96) 82%, var(--surface-elevated) 18%);
  }

  .number-step-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    min-height: 0;
    color: var(--text-secondary);
    transition: background var(--transition), color var(--transition);
  }

  .number-step-button + .number-step-button {
    border-top: 1px solid color-mix(in srgb, var(--border) 76%, rgba(255, 255, 255, 0.04) 24%);
  }

  .number-step-button:hover:not(:disabled) {
    background: color-mix(in srgb, var(--surface-hover) 66%, rgba(7, 16, 29, 0.94) 34%);
    color: var(--text-primary);
  }

  .number-step-button:disabled {
    color: color-mix(in srgb, var(--text-muted) 78%, rgba(255, 255, 255, 0.1) 22%);
    cursor: not-allowed;
  }
</style>
