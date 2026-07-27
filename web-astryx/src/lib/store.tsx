import { createContext, useContext, useState, useCallback, type ReactNode } from "react";
import type { Notification, Settings, Show } from "./types";

interface AppStore {
  shows: Show[];
  setShows: (shows: Show[]) => void;
  settings: Settings | null;
  setSettings: (settings: Settings | null) => void;
  notifications: Notification[];
  addNotification: (message: string, type?: string, duration?: number) => void;
  removeNotification: (id: number) => void;
}

const defaultSettings: Settings = {
  sonarr_url: "",
  sonarr_api_key: "",
  default_filler_action: "unmonitor",
  default_mixed_canon_action: "monitor",
  default_anime_canon_action: "monitor",
  sync_interval: "6h",
  enable_anilist_artwork: false,
  sonarr_add_root_folder: "",
  sonarr_add_quality_profile_id: 0,
  sonarr_add_monitor_mode: "all",
  sonarr_add_season_folder: true,
  afl_show_cache_refreshed_at: null,
  afl_show_cache_count: 0,
  ntfy_enabled: false,
  ntfy_server_url: "",
  ntfy_topic: "",
  ntfy_token_set: false,
  artwork_default_source: "anilist",
  tmdb_api_key: "",
  tmdb_api_key_set: false,
  fanart_api_key: "",
  fanart_api_key_set: false,
};

const AppContext = createContext<AppStore | null>(null);

let notifId = 0;
let lastErrorAt = 0;

export function AppProvider({ children }: { children: ReactNode }) {
  const [shows, setShows] = useState<Show[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const addNotification = useCallback(
    (message: string, type: string = "info", duration: number = 4000) => {
      const id = ++notifId;
      setNotifications((n) => {
        // Dedup: skip if an identical message+type is already showing.
        if (n.some((x) => x.message === message && x.type === type)) {
          return n;
        }
        // Debounce errors: when the backend is unreachable, multiple
        // parallel API calls fail simultaneously, each with its own
        // catch block. Suppress subsequent errors within 800ms of the
        // last one to avoid a toast storm — the first error is enough.
        if (type === "error" && lastErrorAt && Date.now() - lastErrorAt < 800) {
          return n;
        }
        if (type === "error") {
          lastErrorAt = Date.now();
        }
        return [...n, { id, message, type, duration }];
      });
      if (duration > 0) {
        setTimeout(() => {
          setNotifications((n) => n.filter((x) => x.id !== id));
        }, duration);
      }
    },
    [],
  );

  const removeNotification = useCallback((id: number) => {
    setNotifications((n) => n.filter((x) => x.id !== id));
  }, []);

  return (
    <AppContext.Provider
      value={{
        shows,
        setShows,
        settings: settings ?? defaultSettings,
        setSettings,
        notifications,
        addNotification,
        removeNotification,
      }}
    >
      {children}
    </AppContext.Provider>
  );
}

export function useAppStore(): AppStore {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useAppStore must be used within AppProvider");
  return ctx;
}
