<script lang="ts">
  import { onDestroy } from 'svelte';
  import { toast } from 'svelte-sonner';
  import { Toaster } from '$lib/components/ui/sonner/index.js';
  import { notifications, removeNotification } from '../lib/stores.js';

  const displayed = new Map<number>();

  const toastTitleMap: Record<string, string> = {
    success: 'Success',
    error: 'Error',
    warning: 'Warning',
    info: 'Notice',
  };

  function showToast(type: string, title: string, message: string, id: number, duration: number) {
    const toastId = `${id}`;
    const opts = {
      id: toastId,
      description: message,
      duration: duration > 0 ? duration : Infinity,
      onDismiss: () => {
        displayed.delete(id);
        removeNotification(id);
      },
    };

    if (type === 'success') return toast.success(title, opts);
    if (type === 'error') return toast.error(title, opts);
    if (type === 'warning') return toast.warning(title, opts);
    if (type === 'info') return toast.info(title, opts);
    return toast(title, opts);
  }

  $effect(() => {
    const nextIds = new Set($notifications.map((notif) => notif.id));

    for (const notif of $notifications) {
      if (displayed.has(notif.id)) continue;

      showToast(
        notif.type,
        toastTitleMap[notif.type] || 'Notice',
        notif.message,
        notif.id,
        notif.duration
      );

      displayed.set(notif.id, notif.id);
    }

    for (const [id] of displayed) {
      if (nextIds.has(id)) continue;
      toast.dismiss(`${id}`);
      displayed.delete(id);
    }
  });

  onDestroy(() => {
    for (const [id] of displayed) {
      toast.dismiss(`${id}`);
    }
    displayed.clear();
  });
</script>

<Toaster />
