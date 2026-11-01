import { createContext, useContext, useState, useCallback, useEffect, useMemo, type ReactNode } from "react";
import { play as cuelumePlay } from "cuelume";
import { applySoundsEnabled } from "./sounds";
import type { ArtworkProvider, Notification, Settings, Show } from "./types";
import { getShows, getSettings, getArtworkProviders } from "./api";

interface AppStore {
  shows: Show[];
  setShows: (shows: Show[]) => void;
  settings: Settings | null;
  setSettings: (settings: Settings | null) => void;
  artworkProviders: ArtworkProvider[];
  addNotification: (message: string, type?: string, duration?: number) => void;
}

interface NotificationStore {
  notifications: Notification[];
  removeNotification: (id: number) => void;
}

const defaultSettings: Settings = {
  sonarr_url: "",
  sonarr_api_key: "",
  default_filler_action: "unmonitor",
  default_mixed_canon_action: "monitor",
  default_anime_canon_action: "monitor",
  sync_interval: "6h",
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
  artwork_default_source: "sonarr",
  tmdb_api_key: "",
  tmdb_api_key_set: false,
  fanart_api_key: "",
  fanart_api_key_set: false,
  ui_sounds_enabled: true,
};

const AppContext = createContext<AppStore | null>(null);
const NotificationContext = createContext<NotificationStore | null>(null);

let notifId = 0;
let lastErrorAt = 0;

export function AppProvider({ children }: { children: ReactNode }) {
  const [shows, setShows] = useState<Show[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [artworkProviders, setArtworkProviders] = useState<ArtworkProvider[]>([]);
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
      // Play a cuelume cue matching the notification type.
      if (type === "success") {
        cuelumePlay("success", { volume: 0.4 });
      } else if (type === "error") {
        cuelumePlay("error", { volume: 0.4 });
      }
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

  // Fetch shows + settings + artwork providers on mount so they're
  // available app-wide. This lets ShowDetailPage show the hero image
  // instantly on direct navigation/refresh (before getShow resolves)
  // via the store fallback, and avoids re-fetching artwork providers
  // on every ShowDetailPage / SettingsPage visit.
  useEffect(() => {
    const aborted = { current: false };
    Promise.allSettled([getShows(), getSettings(), getArtworkProviders()]).then(
      ([showsRes, settingsRes, providersRes]) => {
        if (aborted.current) return;
        if (showsRes.status === "fulfilled") setShows(showsRes.value || []);
        if (settingsRes.status === "fulfilled") {
          const s = settingsRes.value ?? null;
          setSettings(s);
          // Apply the sounds preference from backend settings.
          applySoundsEnabled(s?.ui_sounds_enabled !== false);
        }
        if (providersRes.status === "fulfilled") {
          setArtworkProviders(providersRes.value?.providers || []);
        }
      },
    );
    return () => { aborted.current = true; };
  }, []);

  // AppStore value — does NOT include notifications array, so pages
  // that only use addNotification (a stable callback) never re-render
  // when toasts appear/disappear.
  const appValue = useMemo<AppStore>(
    () => ({
      shows,
      setShows,
      settings: settings ?? defaultSettings,
      setSettings,
      artworkProviders,
      addNotification,
    }),
    [shows, settings, artworkProviders, addNotification],
  );

  // Notification value — only consumed by ToastViewport, so notification
  // changes don't trigger re-renders of page components.
  const notifValue = useMemo<NotificationStore>(
    () => ({ notifications, removeNotification }),
    [notifications, removeNotification],
  );

  return (
    <AppContext.Provider value={appValue}>
      <NotificationContext.Provider value={notifValue}>
        {children}
      </NotificationContext.Provider>
    </AppContext.Provider>
  );
}

export function useAppStore(): AppStore {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useAppStore must be used within AppProvider");
  return ctx;
}

export function useNotifications(): NotificationStore {
  const ctx = useContext(NotificationContext);
  if (!ctx) throw new Error("useNotifications must be used within AppProvider");
  return ctx;
}
