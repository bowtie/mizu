import { Toast } from "@astryxdesign/core/Toast";
import { VStack } from "@astryxdesign/core/VStack";
import { Check, CircleAlert, Info } from "lucide-react";
import { useNotifications } from "../lib/store";

export function ToastViewport() {
  const { notifications, removeNotification } = useNotifications();

  if (notifications.length === 0) return null;

  return (
    <VStack
      gap={2}
      className="toast-viewport"
    >
      {notifications.map((n) => {
        const icon =
          n.type === "success" ? <Check size={20} /> :
          n.type === "error" ? <CircleAlert size={20} /> :
          <Info size={20} />;

        // Split "Title: detail" into a bold title + lighter detail.
        const colonIdx = n.message.indexOf(": ");
        const title = colonIdx > 0 ? n.message.slice(0, colonIdx) : n.message;
        const detail = colonIdx > 0 ? n.message.slice(colonIdx + 2) : null;

        return (
          <div key={n.id} data-notification-type={n.type}>
            <Toast
              body={
                <span style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                  <span style={{ flexShrink: 0, display: "flex" }}>{icon}</span>
                  <span style={{ flex: 1, minWidth: 0 }}>
                    <span style={{ fontWeight: 700, fontSize: "1.05em" }}>{title}</span>
                    {detail && (
                      <span style={{ display: "block", fontWeight: 400, opacity: 0.85, fontSize: "0.95em" }}>{detail}</span>
                    )}
                  </span>
                </span>
              }
              type={n.type === "error" ? "error" : "info"}
              isAutoHide={n.duration > 0}
              autoHideDuration={n.duration > 0 ? n.duration : 5000}
              onDismiss={() => removeNotification(n.id)}
            />
          </div>
        );
      })}
    </VStack>
  );
}
