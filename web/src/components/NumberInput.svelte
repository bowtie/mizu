<script lang="ts">
  import { onDestroy } from 'svelte';
  import { Minus, Plus } from 'lucide-svelte';

  interface Props {
    id?: string;
    value?: number;
    min?: number;
    max?: number;
    step?: number;
    defaultValue?: number;
    disabled?: boolean;
    className?: string;
  }

  let {
    id = undefined,
    value = $bindable(0),
    min = 0,
    max = Number.POSITIVE_INFINITY,
    step = 1,
    defaultValue = 0,
    disabled = false,
    className = '',
  }: Props = $props();

  const HOLD_DELAY_MS = 325;
  const HOLD_INTERVAL_MS = 85;

  let holdTimeout: ReturnType<typeof setTimeout> | null = null;
  let holdInterval: ReturnType<typeof setInterval> | null = null;

  function asNumber(nextValue: string | number, fallback = 0): number {
    const numeric = Number(nextValue);
    return Number.isNaN(numeric) ? fallback : numeric;
  }

  function clamp(nextValue: string | number, fallback = 0): number {
    const numeric = Number(nextValue);
    const minValue = asNumber(min, 0);
    const maxValue = asNumber(max, Number.POSITIVE_INFINITY);
    if (Number.isNaN(numeric)) return fallbackValue();
    return Math.min(Math.max(numeric, minValue), maxValue);
  }

  function fallbackValue(): number {
    return clamp(defaultValue);
  }

  function updateValue(nextValue: string | number): void {
    value = clamp(nextValue);
  }

  function handleInput(event: Event & { currentTarget: HTMLInputElement }): void {
    const raw = event.currentTarget.value;
    if (raw === '') {
      value = fallbackValue();
      return;
    }

    updateValue(raw);
  }

  function adjust(diff: number): void {
    if (disabled) return;
    updateValue(asNumber(value, 0) + asNumber(diff, 1));
  }

  function handleBlur(): void {
    value = clamp(value);
  }

  function clearHold(): void {
    if (holdTimeout) {
      clearTimeout(holdTimeout);
      holdTimeout = null;
    }

    if (holdInterval) {
      clearInterval(holdInterval);
      holdInterval = null;
    }
  }

  function startHold(diff: number): void {
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

<div class={`flex w-full max-w-44 items-center rounded-lg border border-surface-700 bg-surface-950 text-surface-50 transition focus-within:border-primary-500/50 focus-within:ring-2 focus-within:ring-primary-500/25 ${disabled ? 'opacity-60' : ''} ${className}`.trim()}>
  <button
    type="button"
    class="flex h-8 w-8 items-center justify-center rounded-l-lg text-surface-400 transition hover:bg-primary-500/10 hover:text-surface-50 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-surface-400"
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
    class="h-8 min-w-0 flex-1 border-0 bg-transparent px-2 text-center text-sm font-semibold text-surface-50 outline-none [appearance:textfield] [-moz-appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
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
    class="flex h-8 w-8 items-center justify-center rounded-r-lg text-surface-400 transition hover:bg-primary-500/10 hover:text-surface-50 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-surface-400"
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
