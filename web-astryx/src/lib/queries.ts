import { useCallback, useEffect, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import * as v from "valibot";
import {
  getBootstrap,
  getShows,
  getShow,
  getEpisodes,
  getSettings,
  getSonarrSeries,
  lookupSonarrSeries,
  getSonarrAddOptions,
  getSyncLogs,
  getArtworkProviders,
  getArtworkRefreshStatus,
  suggestAFL,
  searchAFL,
  createShow,
  updateShow,
  deleteShow,
  confirmMatch,
  updateSettings,
  triggerSync,
  refreshArtwork,
  setArtworkSource,
  refreshAllArtwork,
  refreshAflCache,
  setEpisodeMonitor,
  testSonarrConnection,
  testNtfyConnection,
  addSonarrSeries,
  markManualMatchPending,
  getAuthStatus,
} from "./api";
import {
  BootstrapSchema,
  ShowSchema,
  EpisodeSchema,
  SettingsSchema,
  SonarrSeriesSchema,
  SonarrAddOptionsSchema,
  SyncLogResponseSchema,
  ArtworkProviderSchema,
  AFLMatchSchema,
  type Show,
  type Settings,
  type SonarrSeries,
  type SyncLogResponse,
  type AFLMatch,
  type ArtworkProvider,
  type SonarrAddOptions,
  type ArtworkRefreshResult,
  type TestConnectionResult,
  type Episode,
} from "./schemas";

// ── Query keys ──

export const queryKeys = {
  bootstrap: ["bootstrap"] as const,
  shows: ["shows"] as const,
  show: (id: string | number) => ["show", id] as const,
  episodes: (showId: string | number) => ["episodes", showId] as const,
  settings: ["settings"] as const,
  authStatus: ["authStatus"] as const,
  sonarrSeries: (matchableOnly?: boolean) => ["sonarrSeries", matchableOnly ?? false] as const,
  sonarrAddOptions: ["sonarrAddOptions"] as const,
  syncLogs: (limit: number, offset: number) => ["syncLogs", limit, offset] as const,
  artworkProviders: ["artworkProviders"] as const,
  artworkRefreshStatus: ["artworkRefreshStatus"] as const,
};

// ── Delayed artwork refetch ──
// Artwork is fetched in the background after a mapping is created or
// auto-matched. A short delay then refetches just that show and patches
// it into the cached shows list, so artwork appears without refetching
// the entire library. Timers are owned per hook instance (tracked in a
// ref Map keyed by showId) so a component unmounting only cancels its
// own pending refetches — never another component's — and multiple
// shows scheduled by one instance are all cleaned up.

export const ARTWORK_REFETCH_DELAY_MS = 5000;

// Per-show refetch state, kept for the life of the hook instance. Each
// schedule bumps the generation; when a timer fires, `timer` is nulled but
// the entry stays, so any in-flight invalidateQueries promise can tell
// whether a newer schedule superseded it before applying its patch.
interface PendingArtworkRefetch {
  timer: ReturnType<typeof setTimeout> | null;
  generation: number;
}

/**
 * Hook that schedules the delayed artwork refetch for a show. Pending
 * timers are tracked per instance in a Map keyed by showId, so
 * re-scheduling the same show cancels the previous timer (dedupe) while
 * different shows keep independent timers. All timers are cancelled on
 * unmount. When a fired timer's invalidateQueries promise settles, the
 * shows-list patch is skipped if the component has unmounted mid-flight
 * or a newer schedule for the same show has superseded this one —
 * applying the stale snapshot then could clobber fresher data.
 */
export function useShowArtworkRefetch() {
  const qc = useQueryClient();
  const pendingTimersRef = useRef<Map<string | number, PendingArtworkRefetch>>(new Map());
  const disposedRef = useRef(false);

  useEffect(() => {
    disposedRef.current = false;
    return () => {
      disposedRef.current = true;
      pendingTimersRef.current.forEach(({ timer }) => {
        if (timer) clearTimeout(timer);
      });
      pendingTimersRef.current.clear();
    };
  }, []);

  return useCallback(
    (showId: string | number) => {
      const existing = pendingTimersRef.current.get(showId);
      const generation = (existing?.generation ?? 0) + 1;
      if (existing?.timer) clearTimeout(existing.timer);

      const timer = setTimeout(() => {
        // Only the newest schedule for this show may refetch.
        const current = pendingTimersRef.current.get(showId);
        if (!current || current.timer !== timer) return;
        current.timer = null; // fired — entry stays as a supersede marker

        qc.invalidateQueries({ queryKey: queryKeys.show(showId) })
          .then(() => {
            // Skip the patch if the component unmounted mid-flight, or if a
            // newer schedule for the same show superseded this refetch — its
            // own refetch will land fresh data, so patching now would only
            // clobber whatever landed since.
            if (disposedRef.current) return;
            const latest = pendingTimersRef.current.get(showId);
            if (latest && latest.generation > generation) return;

            const updated = qc.getQueryData<Show>(queryKeys.show(showId));
            if (updated) {
              qc.setQueryData<Show[]>(queryKeys.shows, (prev) =>
                prev ? prev.map((s) => (s.id === showId ? { ...s, ...updated } : s)) : prev,
              );
            }
          })
          .catch(() => {});
      }, ARTWORK_REFETCH_DELAY_MS);
      pendingTimersRef.current.set(showId, { timer, generation });
    },
    [qc],
  );
}

// ── Queries ──

export function useBootstrap() {
  return useQuery({
    queryKey: queryKeys.bootstrap,
    queryFn: async () => v.parse(BootstrapSchema, await getBootstrap()),
  });
}

// Auth status is cheap and gates the whole app, so keep it fresh for a
// minute; LoginPage writes the post-login state directly via setQueryData.
export function useAuthStatus() {
  return useQuery({
    queryKey: queryKeys.authStatus,
    queryFn: getAuthStatus,
    staleTime: 60_000,
    retry: false,
  });
}

export function useShows() {
  return useQuery({
    queryKey: queryKeys.shows,
    queryFn: async () => v.parse(v.array(ShowSchema), await getShows()),
  });
}

export function useShow(id: string | number) {
  return useQuery({
    queryKey: queryKeys.show(id),
    queryFn: async () => v.parse(ShowSchema, await getShow(id)),
    enabled: !!id,
  });
}

export function useEpisodes(showId: string | number) {
  return useQuery({
    queryKey: queryKeys.episodes(showId),
    queryFn: async () => v.parse(v.array(EpisodeSchema), await getEpisodes(showId)),
    enabled: !!showId,
  });
}

export function useSettings() {
  return useQuery({
    queryKey: queryKeys.settings,
    queryFn: async () => v.parse(SettingsSchema, await getSettings()),
  });
}

export function useSonarrSeries(matchableOnly: boolean = false) {
  return useQuery({
    queryKey: queryKeys.sonarrSeries(matchableOnly),
    queryFn: async () => v.parse(v.array(SonarrSeriesSchema), await getSonarrSeries(matchableOnly)),
  });
}

export function useSonarrAddOptions() {
  return useQuery({
    queryKey: queryKeys.sonarrAddOptions,
    queryFn: async () => v.parse(SonarrAddOptionsSchema, await getSonarrAddOptions()),
  });
}

export function useSyncLogs(limit: number = 12, offset: number = 0) {
  return useQuery({
    queryKey: queryKeys.syncLogs(limit, offset),
    queryFn: async () => v.parse(SyncLogResponseSchema, await getSyncLogs(limit, offset)),
  });
}

export function useArtworkProviders() {
  return useQuery({
    queryKey: queryKeys.artworkProviders,
    queryFn: async () =>
      v.parse(v.array(ArtworkProviderSchema), (await getArtworkProviders()).providers),
  });
}

export function useArtworkRefreshStatus() {
  return useQuery({
    queryKey: queryKeys.artworkRefreshStatus,
    queryFn: async () => getArtworkRefreshStatus(),
    refetchInterval: 2000,
    enabled: false, // only poll when manually triggered
  });
}

// ── Mutations ──

export function useCreateShow() {
  const qc = useQueryClient();
  const scheduleArtworkRefetch = useShowArtworkRefetch();
  return useMutation({
    mutationFn: createShow,
    onSuccess: (show) => {
      qc.invalidateQueries({ queryKey: queryKeys.show(show.id) });
      qc.invalidateQueries({ queryKey: queryKeys.episodes(show.id) });
      qc.invalidateQueries({ queryKey: queryKeys.shows });
      qc.invalidateQueries({ queryKey: queryKeys.bootstrap });
      scheduleArtworkRefetch(show.id);
    },
  });
}

export function useUpdateShow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string | number; data: Partial<Show> }) =>
      updateShow(id, data),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: queryKeys.shows });
      qc.invalidateQueries({ queryKey: queryKeys.show(vars.id) });
      qc.invalidateQueries({ queryKey: queryKeys.episodes(vars.id) });
    },
  });
}

export function useDeleteShow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteShow,
    onSuccess: (_data, showId) => {
      qc.removeQueries({ queryKey: queryKeys.show(showId) });
      qc.removeQueries({ queryKey: queryKeys.episodes(showId) });
      qc.invalidateQueries({ queryKey: queryKeys.shows });
      qc.invalidateQueries({ queryKey: queryKeys.bootstrap });
    },
  });
}

export function useConfirmMatch() {
  const qc = useQueryClient();
  const scheduleArtworkRefetch = useShowArtworkRefetch();
  return useMutation({
    mutationFn: confirmMatch,
    onSuccess: (show) => {
      qc.invalidateQueries({ queryKey: queryKeys.show(show.id) });
      qc.invalidateQueries({ queryKey: queryKeys.episodes(show.id) });
      qc.invalidateQueries({ queryKey: queryKeys.shows });
      qc.invalidateQueries({ queryKey: queryKeys.bootstrap });
      scheduleArtworkRefetch(show.id);
    },
  });
}

export function useUpdateSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.settings });
      qc.invalidateQueries({ queryKey: queryKeys.bootstrap });
    },
  });
}

export function useTriggerSync() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: triggerSync,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["syncLogs"] });
    },
  });
}

export function useRefreshArtwork() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ showId, source }: { showId: string | number; source?: string }) =>
      refreshArtwork(showId, source),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: queryKeys.show(vars.showId) });
    },
  });
}

export function useSetArtworkSource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ showId, source }: { showId: string | number; source: string }) =>
      setArtworkSource(showId, source),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: queryKeys.show(vars.showId) });
    },
  });
}

export function useRefreshAllArtwork() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: refreshAllArtwork,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.shows });
      qc.invalidateQueries({ queryKey: queryKeys.bootstrap });
    },
  });
}

export function useRefreshAflCache() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: refreshAflCache,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.settings });
    },
  });
}

export function useSetEpisodeMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      showId,
      epId,
      monitored,
    }: {
      showId: string | number;
      epId: string | number;
      monitored: boolean;
    }) => setEpisodeMonitor(showId, epId, monitored),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: queryKeys.episodes(vars.showId) });
    },
  });
}

export function useTestSonarrConnection() {
  return useMutation({
    mutationFn: ({ url, apiKey }: { url: string; apiKey: string }) =>
      testSonarrConnection(url, apiKey),
  });
}

export function useTestNtfyConnection() {
  return useMutation({
    mutationFn: testNtfyConnection,
  });
}

export function useAddSonarrSeries() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: addSonarrSeries,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sonarrSeries"] });
    },
  });
}

export function useMarkManualMatchPending() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: markManualMatchPending,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sonarrSeries"] });
    },
  });
}

// ── AFL queries (enabled on demand) ──

export function useSuggestAFL(seriesId: string | number) {
  return useQuery({
    queryKey: ["aflSuggest", seriesId],
    queryFn: async () => v.parse(v.array(AFLMatchSchema), await suggestAFL(seriesId)),
    enabled: false,
  });
}

export function useSearchAFL() {
  return useMutation({
    mutationFn: async ({ query, seriesId }: { query: string; seriesId?: string | number | null }) =>
      v.parse(v.array(AFLMatchSchema), await searchAFL(query, seriesId)),
  });
}

export function useLookupSonarrSeries() {
  return useMutation({
    mutationFn: async ({ query }: { query: string }) =>
      v.parse(v.array(SonarrSeriesSchema), await lookupSonarrSeries(query)),
  });
}

// ── Re-export types for convenience ──

export type {
  Show,
  Settings,
  SonarrSeries,
  SyncLogResponse,
  AFLMatch,
  ArtworkProvider,
  SonarrAddOptions,
  ArtworkRefreshResult,
  TestConnectionResult,
  Episode,
};
