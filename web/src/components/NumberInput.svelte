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

<div class="ui-field-shell number-input" class:is-disabled={disabled}>
  <input
    {id}
    {name}
    type="text"
    inputmode="numeric"
    pattern="[0-9]*"
    role="spinbutton"
    class="ui-field-control number-input-field"
    bind:value={draftValue}
    aria-label={ariaLabel}
    aria-valuenow={value}
    aria-valuemin={min}
    aria-valuemax={Number.isFinite(max) ? max : undefined}
    aria-disabled={disabled ? 'true' : undefined}
    {disabled}
    on:input={handleInput}
    on:focus={handleFocus}
    on:blur={handleBlur}
    on:keydown={handleKeydown}
  />
  <div class="number-input-stepper">
    <button
      type="button"
      class="ui-field-button number-step-button"
      aria-label="Increase value"
      aria-controls={id}
      on:click={() => adjust(1)}
      disabled={disabled || value >= max}
    >
      <ChevronUp size={14} />
    </button>
    <button
      type="button"
      class="ui-field-button number-step-button"
      aria-label="Decrease value"
      aria-controls={id}
      on:click={() => adjust(-1)}
      disabled={disabled || value <= min}
    >
      <ChevronDown size={14} />
    </button>
  </div>
</div>

<style>
  .number-input {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 32px;
    width: 100%;
    overflow: hidden;
  }

  .number-input.is-disabled {
    opacity: 0.72;
  }

  .number-input-field {
    min-width: 0;
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
    text-align: left;
  }

  .number-input-field:disabled {
    cursor: not-allowed;
  }

  .number-input-stepper {
    display: grid;
    grid-template-rows: 1fr 1fr;
    border-left: 1px solid var(--divider-border);
  }

  .number-step-button {
    width: 100%;
    min-height: 0;
    border-radius: 0;
  }

  .number-step-button + .number-step-button {
    border-top: 1px solid var(--divider-border);
  }
</style>
