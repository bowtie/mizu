<script>
  import { onDestroy } from 'svelte';
  import { Toaster, toast } from '@foxui/core';
  import { notifications, removeNotification } from '../lib/stores.js';

  const displayed = new Map();

  const toastTitleMap = {
    success: 'Success',
    error: 'Error',
    warning: 'Warning',
    info: 'Notice',
  };

  function showToast(type, title, options) {
    if (type === 'success') return toast.success(title, options);
    if (type === 'error') return toast.error(title, options);
    if (type === 'info') return toast.info(title, options);
    return toast(title, options);
  }

  $: {
    const nextIds = new Set($notifications.map((notif) => notif.id));

    for (const notif of $notifications) {
      if (displayed.has(notif.id)) continue;

      const toastId = showToast(notif.type, toastTitleMap[notif.type] || 'Notice', {
        id: String(notif.id),
        description: notif.message,
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

<Toaster colors={{ default: 'base', success: 'green', error: 'rose', info: 'blue' }} />
