<script>
  export let size = 'default'; // 'default', 'lg'
  export let center = false;
  export let message = '';
</script>

{#if center}
  <div class="spinner-center" role="status" aria-live="polite" aria-busy="true">
    <div class="spinner-stack">
      <div class="spinner" class:spinner-lg={size === 'lg'}></div>
      {#if message}
        <p class="spinner-message">{message}</p>
      {/if}
    </div>
  </div>
{:else}
  <span class="spinner" class:spinner-lg={size === 'lg'}></span>
{/if}

<style>
  .spinner-stack {
    display: grid;
    justify-items: center;
    align-items: center;
    gap: 12px;
    text-align: center;
  }

  .spinner-message {
    color: var(--text-secondary);
    font-size: 13px;
    line-height: 1.4;
    text-align: center;
  }
</style>

