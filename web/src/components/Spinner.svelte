<script>
  export let size = 'default'; // 'default', 'lg'
  export let center = false;
  export let message = '';
</script>

{#if center}
  <div class="spinner-center">
    <div>
      <div class="spinner" class:spinner-lg={size === 'lg'}></div>
      {#if message}
        <p style="margin-top:12px;color:var(--text-secondary);font-size:13px;text-align:center">{message}</p>
      {/if}
    </div>
  </div>
{:else}
  <span class="spinner" class:spinner-lg={size === 'lg'}></span>
{/if}

