import { create } from "zustand";

// ── Notification store (Zustand) ──
// Only notifications live here. Shows, settings, and artwork providers
// are managed by TanStack Query (see queries.ts).

export interface Notification {
  id: number;
  message: string;
  type: string;
  duration: number;
}

interface NotificationState {
  notifications: Notification[];
  addNotification: (message: string, type?: string, duration?: number) => void;
  removeNotification: (id: number) => void;
}

let notifId = 0;
let lastErrorAt = 0;

// Lazy-load cuelume — the `play` function is only called when a
// notification appears, never on first paint.
let cuelumePlay: ((sound: string, opts?: { volume?: number }) => void) | null = null;
void import("cuelume")
  .then((m) => {
    cuelumePlay = m.play as (sound: string, opts?: { volume?: number }) => void;
  })
  .catch(() => {
    /* cuelume optional */
  });

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  addNotification: (message: string, type: string = "info", duration: number = 4000) => {
    const id = ++notifId;
    set((state) => {
      // Dedup: skip if an identical message+type is already showing.
      if (state.notifications.some((x) => x.message === message && x.type === type)) {
        return state;
      }
      // Debounce errors: when the backend is unreachable, multiple
      // parallel API calls fail simultaneously. Suppress subsequent
      // errors within 800ms of the last one to avoid a toast storm.
      if (type === "error" && lastErrorAt && Date.now() - lastErrorAt < 800) {
        return state;
      }
      if (type === "error") {
        lastErrorAt = Date.now();
      }
      return { notifications: [...state.notifications, { id, message, type, duration }] };
    });
    // Play a cuelume cue matching the notification type.
    if (cuelumePlay) {
      if (type === "success") {
        cuelumePlay("success", { volume: 0.4 });
      } else if (type === "error") {
        cuelumePlay("error", { volume: 0.4 });
      }
    }
    if (duration > 0) {
      setTimeout(() => {
        set((state) => ({
          notifications: state.notifications.filter((x) => x.id !== id),
        }));
      }, duration);
    }
  },
  removeNotification: (id: number) =>
    set((state) => ({
      notifications: state.notifications.filter((x) => x.id !== id),
    })),
}));

// Convenience hook for components that only need addNotification.
export function useAddNotification() {
  return useNotificationStore((s) => s.addNotification);
}
