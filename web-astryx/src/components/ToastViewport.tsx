import { Toast } from '@astryxdesign/core/Toast';
import { VStack } from '@astryxdesign/core/VStack';
import { useAppStore } from '../lib/store';

export function ToastViewport() {
  const { notifications, removeNotification } = useAppStore();

  if (notifications.length === 0) return null;

  return (
    <VStack
      gap={2}
      style={{
        position: 'fixed',
        bottom: 'var(--spacing-6)',
        right: 'var(--spacing-6)',
        zIndex: 9999,
        maxWidth: '400px',
      }}
    >
      {notifications.map((n) => (
        <Toast
          key={n.id}
          body={n.message}
          type={n.type === 'error' ? 'error' : 'info'}
          isAutoHide={n.duration > 0}
          autoHideDuration={n.duration > 0 ? n.duration : 5000}
          onDismiss={() => removeNotification(n.id)}
        />
      ))}
    </VStack>
  );
}
