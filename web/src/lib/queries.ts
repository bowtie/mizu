import { useQuery, useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import * as v from "valibot";
import {
  getShows,
  getShow,
  getEpisodes,
  getSettings,
  getSonarrSeries,
  getArtworkProviders,
  getSyncLogs,
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
  syncLogs: (limit: number, offset: number) => ["syncLogs", limit, offset] as const,
  artworkProviders: ["artworkProviders"] as const,
};

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
    mutationFn: updateSettings,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.settings });
    },
  });
}

export function useTriggerSync() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: triggerSync,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["syncLogs"] });
    },
  });
}

// ── Re-export types for convenience ──

export type { Show, Settings, SonarrSeries, ArtworkProvider, Episode };
