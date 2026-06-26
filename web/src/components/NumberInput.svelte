<script>
  import { onDestroy } from 'svelte';
  import { Minus, Plus } from 'lucide-svelte';

  export let id = undefined;
  export let value = 0;
  export let min = 0;
  export let max = Number.POSITIVE_INFINITY;
  export let step = 1;
  export let defaultValue = 0;
  export let disabled = false;
  export let className = '';

  const HOLD_DELAY_MS = 325;
  const HOLD_INTERVAL_MS = 85;

  let holdTimeout = null;
  let holdInterval = null;

  function asNumber(nextValue, fallback = 0) {
    const numeric = Number(nextValue);
    return Number.isNaN(numeric) ? fallback : numeric;
  }

  function clamp(nextValue) {
    const numeric = Number(nextValue);
    const minValue = asNumber(min, 0);
    const maxValue = asNumber(max, Number.POSITIVE_INFINITY);
    if (Number.isNaN(numeric)) return fallbackValue();
    return Math.min(Math.max(numeric, minValue), maxValue);
  }

  function fallbackValue() {
    return clamp(defaultValue);
  }

  function updateValue(nextValue) {
    value = clamp(nextValue);
  }

  function handleInput(event) {
    const raw = event.currentTarget.value;
    if (raw === '') {
      value = fallbackValue();
      return;
    }

    updateValue(raw);
  }

  function adjust(diff) {
    if (disabled) return;
    updateValue(asNumber(value, 0) + asNumber(diff, 1));
  }

  function handleBlur() {
    value = clamp(value);
  }

  function clearHold() {
    if (holdTimeout) {
      clearTimeout(holdTimeout);
      holdTimeout = null;
    }

    if (holdInterval) {
      clearInterval(holdInterval);
      holdInterval = null;
    }
  }

  function startHold(diff) {
    if (disabled) return;

    adjust(diff);

    holdTimeout = setTimeout(() => {
      holdInterval = setInterval(() => {
        adjust(diff);
      }, HOLD_INTERVAL_MS);
    }, HOLD_DELAY_MS);
  }

  onDestroy(clearHold);
</script>

<div class={`flex w-full max-w-44 items-center rounded-2xl border border-blue-500/18 bg-blue-950/12 text-blue-200 shadow-[inset_0_1px_0_rgba(96,165,250,0.08)] transition focus-within:border-blue-400/45 focus-within:ring-2 focus-within:ring-blue-500/25 ${disabled ? 'opacity-60' : ''} ${className}`.trim()}>
  <button
    type="button"
    class="flex h-11 w-11 items-center justify-center rounded-l-2xl text-blue-300 transition hover:bg-blue-500/8 hover:text-blue-100 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-blue-300"
    onclick={(event) => event.preventDefault()}
    onpointerdown={() => startHold(-step)}
    onpointerup={clearHold}
    onpointerleave={clearHold}
    onpointercancel={clearHold}
    disabled={disabled || asNumber(value, 0) <= asNumber(min, 0)}
    aria-label="Decrease value"
  >
    <Minus size={16} />
  </button>

  <input
    {id}
    type="number"
    class="h-11 min-w-0 flex-1 border-0 bg-transparent px-2 text-center text-lg font-semibold text-base-50 outline-none [appearance:textfield] [-moz-appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
    bind:value
    {min}
    {max}
    {step}
    {disabled}
    inputmode="numeric"
    autocomplete="off"
    oninput={handleInput}
    onblur={handleBlur}
    onkeydown={clearHold}
  />

  <button
    type="button"
    class="flex h-11 w-11 items-center justify-center rounded-r-2xl text-blue-300 transition hover:bg-blue-500/8 hover:text-blue-100 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-blue-300"
    onclick={(event) => event.preventDefault()}
    onpointerdown={() => startHold(step)}
    onpointerup={clearHold}
    onpointerleave={clearHold}
    onpointercancel={clearHold}
    disabled={disabled || asNumber(value, 0) >= asNumber(max, Number.POSITIVE_INFINITY)}
    aria-label="Increase value"
  >
    <Plus size={16} />
  </button>
</div>
