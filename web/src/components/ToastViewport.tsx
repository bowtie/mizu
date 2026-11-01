import { VStack } from "@astryxdesign/core/VStack";
import { Check, CircleAlert, Info } from "lucide-react";
import { lazy, Suspense } from "react";
import { useNotificationStore } from "../lib/store";

const Toast = lazy(() => import("@astryxdesign/core/Toast").then((m) => ({ default: m.Toast })));

export function ToastViewport() {
  const { notifications, removeNotification } = useNotificationStore();

  if (notifications.length === 0) return null;

  return (
    <VStack gap={2} className="toast-viewport">
      {notifications.map((n) => {
        const icon =
          n.type === "success" ? (
            <Check size={20} />
          ) : n.type === "error" ? (
            <CircleAlert size={20} />
          ) : (
            <Info size={20} />
          );

        // Split "Title: detail" into a bold title + lighter detail.
        const colonIdx = n.message.indexOf(": ");
        const title = colonIdx > 0 ? n.message.slice(0, colonIdx) : n.message;
        const detail = colonIdx > 0 ? n.message.slice(colonIdx + 2) : null;

        return (
          <div key={n.id} data-notification-type={n.type}>
            <Suspense fallback={null}>
              <Toast
                body={
                  <span style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                    <span style={{ flexShrink: 0, display: "flex" }}>{icon}</span>
                    <span style={{ flex: 1, minWidth: 0 }}>
                      <span>{title}</span>
                      {detail && (
                        <span
                          style={{
                            display: "block",
                            opacity: 0.7,
                            fontSize: "0.85em",
                          }}
                        >
                          {detail}
                        </span>
                      )}
                    </span>
                  </span>
                }
                type={n.type === "error" ? "error" : "info"}
                isAutoHide={n.duration > 0}
                autoHideDuration={n.duration > 0 ? n.duration : 5000}
                onDismiss={() => removeNotification(n.id)}
              />
            </Suspense>
          </div>
        );
      })}
    </VStack>
  );
}
