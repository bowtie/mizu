import { Toast } from "@astryxdesign/core/Toast";
import { VStack } from "@astryxdesign/core/VStack";
import { useAppStore } from "../lib/store";

export function ToastViewport() {
  const { notifications, removeNotification } = useAppStore();

  if (notifications.length === 0) return null;

  return (
    <VStack
      gap={2}
      className="toast-viewport"
    >
      {notifications.map((n) => (
        <div key={n.id} data-notification-type={n.type}>
          <Toast
            body={n.message}
            type={n.type === "error" ? "error" : "info"}
            isAutoHide={n.duration > 0}
            autoHideDuration={n.duration > 0 ? n.duration : 5000}
            onDismiss={() => removeNotification(n.id)}
          />
        </div>
      ))}
    </VStack>
  );
}
