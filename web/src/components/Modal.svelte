<script>
  import { createEventDispatcher, tick } from 'svelte';
  import { X } from 'lucide-svelte';
  import Surface from './ui/Surface.svelte';
  import Button from './ui/Button.svelte';

  export let title = '';
  export let show = false;
  export let size = 'default'; // 'default', 'large'

  const dispatch = createEventDispatcher();
  const titleId = `modal-title-${Math.random().toString(36).slice(2, 10)}`;

  let modalElement;
  let previousActiveElement = null;

  function close() {
    dispatch('close');
  }

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) {
      close();
    }
  }

  function handleKeydown(e) {
    if (show && e.key === 'Escape') {
      close();
    }
  }

  $: if (show) {
    previousActiveElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    tick().then(() => {
      modalElement?.focus();
    });
  } else if (previousActiveElement) {
    previousActiveElement.focus();
    previousActiveElement = null;
  }
</script>

{#if show}
  <svelte:window on:keydown={handleKeydown} />
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click={handleBackdropClick}>
    <Surface
      bind:this={modalElement}
      class="modal"
      variant="default"
      padding="none"
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      tabindex="-1"
      style={size === 'large' ? 'max-width:720px' : ''}
    >
      <div class="modal-header">
        <h3 id={titleId}>{title}</h3>
        <Button class="modal-close" variant="ghost" size="icon" aria-label="Close dialog" onclick={close}>
          <X size={16} />
        </Button>
      </div>
      <div class="modal-body">
        <slot />
      </div>
      {#if $$slots.footer}
        <div class="modal-footer">
          <slot name="footer" />
        </div>
      {/if}
    </Surface>
  </div>
{/if}

