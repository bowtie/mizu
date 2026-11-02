import { useEffect, useRef } from "react";
import {
  queryOptions,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import * as v from "valibot";
import {
  getShows,
  getShow,
  getEpisodes,
  getSettings,
  getSonarrSeries,
  getArtworkProviders,
  getSyncLogs,
  getSonarrAddOptions,
  getAflCacheRefreshStatus,
  refreshAflCache,
  addSonarrSeries,
  markManualMatchPending,
  confirmMatch,
  errorMessage,
  updateShow,
  deleteShow,
  updateSettings,
  triggerSync,
  setEpisodeMonitor,
  getAuthStatus,
} from "./api";
import {
  ShowSchema,
  EpisodeSchema,
  SettingsSchema,
  SonarrSeriesSchema,
  ArtworkProviderSchema,
  SyncLogResponseSchema,
  SonarrAddOptionsSchema,
  type Show,
  type Settings,
  type SonarrSeries,
  type ArtworkProvider,
  type Episode,
} from "./schemas";
import type { ShowUpdate } from "./types";

// TanStack Query is the only client-side cache: every server read goes
// through a query below, and every write updates or invalidates the keys
// it affects. api.ts only talks HTTP.

// ── Query keys ──
// Show ids arrive as numbers from the API and as strings from the URL;
// keys use the string form so both address the same cache entry.

export const queryKeys = {
  shows: ["shows"] as const,
  show: (id: string | number) => ["show", String(id)] as const,
  episodes: (showId: string | number) => ["episodes", String(showId)] as const,
  settings: ["settings"] as const,
  authStatus: ["authStatus"] as const,
  sonarrSeries: (matchableOnly?: boolean) => ["sonarrSeries", matchableOnly ?? false] as const,
  sonarrAddOptions: ["sonarrAddOptions"] as const,
  syncLogs: (limit: number, offset: number) => ["syncLogs", limit, offset] as const,
  syncLogPages: (pageSize: number) => ["syncLogs", "pages", pageSize] as const,
  artworkProviders: ["artworkProviders"] as const,
  aflCacheRefreshStatus: ["aflCacheRefreshStatus"] as const,
};

// ── Shared query definitions ──
// For reads that are also fetched imperatively (queryClient.fetchQuery),
// so both paths parse and cache the response the same way.

export const settingsQuery = queryOptions({
  queryKey: queryKeys.settings,
  queryFn: async () => v.parse(SettingsSchema, await getSettings()),
});

export const sonarrSeriesQuery = (matchableOnly = false) =>
  queryOptions({
    queryKey: queryKeys.sonarrSeries(matchableOnly),
    queryFn: async () => v.parse(v.array(SonarrSeriesSchema), await getSonarrSeries(matchableOnly)),
  });

export const sonarrAddOptionsQuery = queryOptions({
  queryKey: queryKeys.sonarrAddOptions,
  queryFn: async () => v.parse(SonarrAddOptionsSchema, await getSonarrAddOptions()),
});

const fetchEpisodes = async (showId: string | number) =>
  v.parse(v.array(EpisodeSchema), await getEpisodes(showId));

// ── Queries ──

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

/** A single show. While it loads, the library list's copy of the show (if
 *  cached) is shown as placeholder data, so opening a show from the library
 *  renders immediately. */
export function useShow(id: string | number | undefined) {
  const qc = useQueryClient();
  return useQuery({
    queryKey: queryKeys.show(id ?? ""),
    queryFn: async () => v.parse(ShowSchema, await getShow(id!)),
    enabled: id != null && id !== "",
    placeholderData: () =>
      qc.getQueryData<Show[]>(queryKeys.shows)?.find((s) => String(s.id) === String(id)),
  });
}

export function useEpisodes(showId: string | number | undefined) {
  return useQuery({
    queryKey: queryKeys.episodes(showId ?? ""),
    queryFn: () => fetchEpisodes(showId!),
    enabled: showId != null && showId !== "",
  });
}

/** Warm the episode list for a show (e.g. on hover in the library) so the
 *  detail page's useEpisodes finds it cached. */
export function prefetchEpisodes(qc: QueryClient, showId: string | number) {
  return qc.prefetchQuery({
    queryKey: queryKeys.episodes(showId),
    queryFn: () => fetchEpisodes(showId),
  });
}

export function useSettings() {
  return useQuery(settingsQuery);
}

export function useSonarrSeries(matchableOnly: boolean = false, enabled: boolean = true) {
  return useQuery({ ...sonarrSeriesQuery(matchableOnly), enabled });
}

export function useSyncLogs(limit: number = 12, offset: number = 0) {
  return useQuery({
    queryKey: queryKeys.syncLogs(limit, offset),
    queryFn: async () => v.parse(SyncLogResponseSchema, await getSyncLogs(limit, offset)),
  });
}

/** Sync history, a page at a time ("Load more"). While a sync is running —
 *  or for a short grace period after one was triggered, before its log row
 *  exists — it re-polls every 2s, dropping to 30s once the run is older than
 *  five minutes. Focus refetches resume tracking after a hidden tab. */
export function useSyncLogPages(pageSize: number, pollUntil: number) {
  return useInfiniteQuery({
    queryKey: queryKeys.syncLogPages(pageSize),
    queryFn: async ({ pageParam }) =>
      v.parse(SyncLogResponseSchema, await getSyncLogs(pageSize, pageParam)),
    initialPageParam: 0,
    getNextPageParam: (last, pages) =>
      last.has_more
        ? (last.next_offset ?? pages.reduce((n, p) => n + (p.items?.length ?? 0), 0))
        : undefined,
    refetchInterval: (query) => {
      const running = query.state.data?.pages
        .flatMap((p) => p.items ?? [])
        .find((log) => log.status === "running");
      if (running) {
        const age = Date.now() - new Date(running.started_at ?? Date.now()).getTime();
        return age < FAST_POLL_WINDOW_MS ? 2000 : SLOW_POLL_MS;
      }
      return Date.now() < pollUntil ? 2000 : false;
    },
  });
}

export function useArtworkProviders() {
  return useQuery({
    queryKey: queryKeys.artworkProviders,
    queryFn: async () =>
      v.parse(v.array(ArtworkProviderSchema), (await getArtworkProviders()).providers),
  });
}

// ── Mutations ──

/** Re-read a show after something changed it on the server (artwork,
 *  rules). Resolves once the show query has refetched. */
export function invalidateShow(qc: QueryClient, showId: string | number) {
  void qc.invalidateQueries({ queryKey: queryKeys.shows });
  return qc.invalidateQueries({ queryKey: queryKeys.show(showId) });
}

export function useUpdateShow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string | number; data: ShowUpdate }) => updateShow(id, data),
    onSuccess: (updated, vars) => {
      // The server answers with the saved (and re-synced) mapping.
      qc.setQueryData(queryKeys.show(vars.id), v.parse(ShowSchema, updated));
      void qc.invalidateQueries({ queryKey: queryKeys.shows });
      void qc.invalidateQueries({ queryKey: queryKeys.episodes(vars.id) });
    },
  });
}

export function useDeleteShow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (showId: string | number) => deleteShow(showId),
    onSuccess: (_data, showId) => {
      qc.removeQueries({ queryKey: queryKeys.show(showId) });
      qc.removeQueries({ queryKey: queryKeys.episodes(showId) });
      void qc.invalidateQueries({ queryKey: queryKeys.shows });
    },
  });
}

/** Toggle one episode's monitored flag in Sonarr, then patch that episode
 *  in the cached list instead of refetching the whole list. */
export function useSetEpisodeMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { showId: string | number; epId: number; monitored: boolean }) =>
      setEpisodeMonitor(vars.showId, vars.epId, vars.monitored),
    onSuccess: (_data, { showId, epId, monitored }) => {
      qc.setQueryData<Episode[]>(queryKeys.episodes(showId), (prev) =>
        prev?.map((ep) => (ep.sonarr_episode_id === epId ? { ...ep, monitored } : ep)),
      );
    },
  });
}

export function useUpdateSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof updateSettings>[0]) => updateSettings(data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.settings });
    },
  });
}

// ── Background refresh jobs ──
// Artwork refresh-all and the AFL show-list refresh run server-side long
// after the triggering request returns; the server exposes a
// {refreshing, last_error} status endpoint for each.

export interface RefreshJobStatus {
  refreshing: boolean;
  last_error?: string;
}

const FAST_POLL_WINDOW_MS = 5 * 60_000;
const SLOW_POLL_MS = 30_000;

interface RefreshJobOptions {
  queryKey: readonly unknown[];
  getStatus: () => Promise<RefreshJobStatus>;
  trigger: () => Promise<unknown>;
  /** Called once when a run (started here or found running) ends. */
  onFinished: (status: RefreshJobStatus) => void;
}

/** Tracks a background refresh: start() triggers it (joining a run already
 *  in progress), then the status is polled — every 2s for five minutes,
 *  then every 30s — until it ends. A run already going when the page opens
 *  is picked up too. Refetch-on-focus resumes polling after a hidden tab. */
function useRefreshJob({ queryKey, getStatus, trigger, onFinished }: RefreshJobOptions) {
  const qc = useQueryClient();
  const startedAt = useRef(0);
  const onFinishedRef = useRef(onFinished);
  useEffect(() => {
    onFinishedRef.current = onFinished;
  });

  const status = useQuery({
    queryKey,
    queryFn: getStatus,
    staleTime: 0,
    retry: 2,
    refetchInterval: (query) => {
      if (!query.state.data?.refreshing) return false;
      if (!startedAt.current) startedAt.current = Date.now();
      return Date.now() - startedAt.current < FAST_POLL_WINDOW_MS ? 2000 : SLOW_POLL_MS;
    },
  });

  const running = !!status.data?.refreshing;
  const wasRunning = useRef(running);
  useEffect(() => {
    if (wasRunning.current && !running && status.data) {
      startedAt.current = 0;
      onFinishedRef.current(status.data);
    }
    wasRunning.current = running;
  }, [running, status.data]);

  const startMutation = useMutation({
    mutationFn: async () => {
      try {
        await trigger();
      } catch (err) {
        // Already running (started elsewhere): just watch that run.
        if (!errorMessage(err).includes("already in progress")) throw err;
      }
    },
    onSuccess: () => {
      startedAt.current = Date.now();
      qc.setQueryData<RefreshJobStatus>(queryKey, { refreshing: true });
    },
  });

  return {
    running: running || startMutation.isPending,
    start: startMutation.mutateAsync,
  };
}

export function useAflCacheRefreshJob(onFinished: (status: RefreshJobStatus) => void) {
  return useRefreshJob({
    queryKey: queryKeys.aflCacheRefreshStatus,
    getStatus: getAflCacheRefreshStatus,
    trigger: refreshAflCache,
    onFinished,
  });
}

// ── Add-anime writes ──
// Each updates the cached series lists so the library and wizard see the
// change without a full refetch.

export function useConfirmMatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof confirmMatch>[0]) => confirmMatch(data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.shows });
      void qc.invalidateQueries({ queryKey: queryKeys.sonarrSeries(true) });
    },
  });
}

export function useAddSonarrSeries() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof addSonarrSeries>[0]) => addSonarrSeries(data),
    onSuccess: (created) => {
      const series = v.parse(SonarrSeriesSchema, created);
      qc.setQueryData<SonarrSeries[]>(queryKeys.sonarrSeries(false), (prev) =>
        [...(prev ?? []).filter((s) => s.id !== series.id), series].sort((a, b) =>
          a.title.localeCompare(b.title),
        ),
      );
    },
  });
}

export function useMarkManualMatchPending() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (seriesId: number) => markManualMatchPending(seriesId),
    onSuccess: (_data, seriesId) => {
      for (const matchableOnly of [false, true]) {
        qc.setQueryData<SonarrSeries[]>(queryKeys.sonarrSeries(matchableOnly), (prev) =>
          prev?.map((s) => (s.id === seriesId ? { ...s, manualMatchPending: true } : s)),
        );
      }
    },
  });
}

export function useTriggerSync() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => triggerSync(),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["syncLogs"] });
    },
  });
}

// ── Re-export types for convenience ──

export type { Show, Settings, SonarrSeries, ArtworkProvider, Episode };
