<script>
  import { notifications, removeNotification } from '../lib/stores.js';

  const icons = {
    success: '&#10003;',
    error: '&#10007;',
    warning: '&#9888;',
    info: '&#8505;',
  };
</script>

<div class="toast-container">
  {#each $notifications as notif (notif.id)}
    <div
      class="toast toast-{notif.type}"
      role={notif.type === 'error' ? 'alert' : 'status'}
      aria-live={notif.type === 'error' ? 'assertive' : 'polite'}
    >
      <span class="toast-icon">{@html icons[notif.type] || icons.info}</span>
      <span class="toast-message">{notif.message}</span>
      <button class="toast-close" aria-label="Dismiss notification" on:click={() => removeNotification(notif.id)}>&times;</button>
    </div>
  {/each}
</div>


