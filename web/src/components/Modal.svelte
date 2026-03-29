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
    if (!show) return;

    if (e.key === 'Escape') {
      close();
      return;
    }

    if (e.key !== 'Tab' || !modalElement) return;

    const focusableSelectors = [
      'a[href]',
      'button:not([disabled])',
      'textarea:not([disabled])',
      'input:not([disabled])',
      'select:not([disabled])',
      '[tabindex]:not([tabindex="-1"])',
    ];

    const focusableElements = Array.from(
      modalElement.querySelectorAll(focusableSelectors.join(','))
    ).filter((element) => {
      return element instanceof HTMLElement && !element.hasAttribute('disabled') && element.tabIndex !== -1;
    });

    if (focusableElements.length === 0) {
      e.preventDefault();
      modalElement.focus();
      return;
    }

    const firstElement = focusableElements[0];
    const lastElement = focusableElements[focusableElements.length - 1];
    const activeElement = document.activeElement;

    if (e.shiftKey && (activeElement === firstElement || activeElement === modalElement)) {
      e.preventDefault();
      lastElement.focus();
      return;
    }

    if (!e.shiftKey && activeElement === lastElement) {
      e.preventDefault();
      firstElement.focus();
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

<svelte:window on:keydown={handleKeydown} />

{#if show}
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

