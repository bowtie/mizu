<script>
  import { onDestroy } from 'svelte';
  import { Toaster, toast } from 'svelte-sonner';
  import { notifications, removeNotification } from '../lib/stores.js';

  const displayed = new Map();

  const toastTypeMap = {
    success: 'success',
    error: 'error',
    warning: 'warning',
    info: 'info',
  };

  $: {
    const nextIds = new Set($notifications.map((notif) => notif.id));

    for (const notif of $notifications) {
      if (displayed.has(notif.id)) continue;

      const showToast = toast[toastTypeMap[notif.type]] ?? toast;
      const toastId = showToast(notif.message, {
        id: String(notif.id),
        duration: notif.duration > 0 ? notif.duration : Infinity,
        onDismiss: () => {
          displayed.delete(notif.id);
          removeNotification(notif.id);
        },
      });

      displayed.set(notif.id, toastId);
    }

    for (const [id, toastId] of displayed) {
      if (nextIds.has(id)) continue;
      toast.dismiss(toastId);
      displayed.delete(id);
    }
  }

  onDestroy(() => {
    for (const [, toastId] of displayed) {
      toast.dismiss(toastId);
    }
    displayed.clear();
  });
</script>

<Toaster
  position="top-right"
  offset="calc(var(--topbar-height) + 12px)"
  mobileOffset="12px"
  richColors={false}
  expand={false}
  closeButton
  toastOptions={{
    unstyled: true,
    classes: {
      toast: 'mizu-toast',
      title: 'mizu-toast-title',
      description: 'mizu-toast-description',
      closeButton: 'mizu-toast-close',
      success: 'is-success',
      error: 'is-error',
      warning: 'is-warning',
      info: 'is-info',
    },
  }}
/>


