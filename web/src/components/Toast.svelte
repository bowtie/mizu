<script lang="ts">
  import { onDestroy } from 'svelte';
  import { toast } from 'svelte-sonner';
  import { Toaster } from '$lib/components/ui/sonner/index.ts';
  import { notifications, removeNotification } from '../lib/stores.ts';

  const displayed = new Map<number, number>();

  function showToast(type: string, message: string, id: number, duration: number) {
    const toastId = `${id}`;
    const opts = {
      id: toastId,
      duration: duration > 0 ? duration : Infinity,
      onDismiss: () => {
        displayed.delete(id);
        removeNotification(id);
      },
    };

    if (type === 'success') return toast.success(message, opts);
    if (type === 'error') return toast.error(message, opts);
    if (type === 'warning') return toast.warning(message, opts);
    if (type === 'info') return toast.info(message, opts);
    return toast(message, opts);
  }

  $effect(() => {
    const nextIds = new Set($notifications.map((notif) => notif.id));

    for (const notif of $notifications) {
      if (displayed.has(notif.id)) continue;

      showToast(notif.type, notif.message, notif.id, notif.duration);
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
