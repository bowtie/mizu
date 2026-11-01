import ky, { HTTPError } from "ky";
import type {
  AFLMatch,
  ArtworkProvider,
  ArtworkRefreshResult,
  Episode,
  MappingCreate,
  Settings,
  Show,
  ShowUpdate,
  SonarrAddOptions,
  SonarrAddPayload,
  SonarrSeries,
  SyncLogResponse,
  TestConnectionResult,
} from "./types";

const BASE = "/api";

// Pre-fired fetch responses from the inline script in index.html.
// The inline script fires fetch() before the JS bundle downloads,
// so API requests start in parallel with the 324KB bundle download.
interface MizuPrefetch {
  bootstrap?: Promise<Response>;
  detailId?: string;
  episodes?: Promise<Response>;
  show?: Promise<Response>;
  series?: Promise<Response>;
}
const prefetch = (globalThis as unknown as { window?: { __mizuPrefetch?: MizuPrefetch } }).window
  ?.__mizuPrefetch;

// ky instance — handles JSON serialization, error parsing, and retries.
// The prefetch system (inline script Responses) and ETag-based episode
// caching still use raw fetch because ky can't consume pre-fired Response
// objects or handle 304 Not Modified.
const api = ky.create({
  prefix: BASE + "/",
  headers: { "Content-Type": "application/json" },
  retry: 0, // TanStack Query handles retries at a higher level
  hooks: {
    beforeError: [
      async (state) => {
        const { error } = state;
        if (error instanceof HTTPError && error.response) {
          try {
            const body = (await error.response.clone().json()) as { error?: string };
            if (body.error) {
              error.message = body.error;
            }
          } catch {
            // response body isn't JSON — keep ky's default message
          }
        }
        return error;
      },
    ],
  },
});

async function request<T>(method: string, path: string, body: unknown = null): Promise<T> {
  const init: Record<string, unknown> = { method };
  if (body) init.json = body;
  return api(path, init).json() as Promise<T>;
}

// Like request(), but reuses a pre-fired Response from the inline
// script if available. The Response is already in-flight (or
// completed) by the time the JS bundle runs — we just read it.
async function requestPrefetched<T>(
  prefetched: Promise<Response> | undefined,
  fallbackPath: string,
): Promise<T> {
  if (prefetched) {
    const res = await prefetched;
    if (res.ok) return res.clone().json() as Promise<T>;
    // If the prefetched response failed, fall through to a fresh request.
    if (res.status === 404) throw new Error("not found");
  }
  return request<T>("GET", fallbackPath);
}

export function getSonarrSeries(matchableOnly: boolean = false): Promise<SonarrSeries[]> {
  const params = new URLSearchParams();
  if (matchableOnly) params.set("matchable_only", "true");
  const suffix = params.size > 0 ? "?" + params.toString() : "";
  // Use the pre-fired Response from the inline script if available
  // (only for the unfiltered list, not the matchable_only variant).
  if (!matchableOnly && prefetch?.series) {
    return requestPrefetched<SonarrSeries[]>(prefetch.series, "/sonarr/series");
  }
  return request<SonarrSeries[]>("GET", "/sonarr/series" + suffix);
}

export function lookupSonarrSeries(query: string): Promise<SonarrSeries[]> {
  const params = new URLSearchParams();
  params.set("q", query || "");
  return request<SonarrSeries[]>("GET", "/sonarr/lookup?" + params.toString());
}

export function addSonarrSeries(data: SonarrAddPayload): Promise<SonarrSeries> {
  return request<SonarrSeries>("POST", "/sonarr/series", data);
}

export function getSonarrAddOptions(): Promise<SonarrAddOptions> {
  return request<SonarrAddOptions>("GET", "/sonarr/add-options");
}

export function markManualMatchPending(seriesId: string | number): Promise<unknown> {
  return request<unknown>("POST", "/sonarr/manual-match-pending", { sonarr_series_id: seriesId });
}

export function testSonarrConnection(url: string, apiKey: string): Promise<TestConnectionResult> {
  return request<TestConnectionResult>("POST", "/sonarr/test", { url, api_key: apiKey });
}

export function getShows(): Promise<Show[]> {
  return request<Show[]>("GET", "/mappings");
}

// Bootstrap — returns shows + settings + artwork providers in one
// response, eliminating 2 round-trips on app mount.
interface BootstrapResponse {
  shows: Show[];
  settings: Settings;
  providers: ArtworkProvider[];
}

export function getBootstrap(): Promise<BootstrapResponse> {
  return requestPrefetched<BootstrapResponse>(prefetch?.bootstrap, "/bootstrap");
}

export function getShow(id: string | number): Promise<Show> {
  const key = String(id);
  // If prefetchShowDetail already resolved and cached the show data,
  // return it directly — avoids double JSON parsing.
  const cachedDetail = detailDataCache.get(key);
  if (cachedDetail?.show) {
    return Promise.resolve(cachedDetail.show);
  }
  // If prefetchShowDetail has an in-flight show promise, use it
  // directly — resolves as soon as the 583B show fetch completes
  // without waiting for the 252KB episodes fetch.
  const inflightDetail = detailPrefetch.get(key);
  if (inflightDetail) {
    return inflightDetail.show;
  }
  // Use the pre-fired Response from the inline script if available.
  const preFired = prefetch?.show && prefetch.detailId === key ? prefetch.show : undefined;
  return requestPrefetched<Show>(preFired, "/mappings/" + key);
}

// Combined detail response — show + episodes. Used internally by
// the prefetch system to provide initial state for ShowDetailPage.
interface ShowDetailResponse {
  show: Show;
  episodes: Episode[];
}

// Individual in-flight promises from prefetchShowDetail — stored
// separately so getShow can resolve as soon as the show-only fetch
// completes without waiting for the (much slower) episodes fetch.
interface DetailPrefetchEntry {
  show: Promise<Show>;
  episodes: Promise<Episode[]>;
}

// In-memory cache for combined detail prefetches — used on direct
// navigation to /shows/:id to avoid 2 separate round-trips.
const detailPrefetch = new Map<string, DetailPrefetchEntry>();
// Synchronous cache for resolved detail data — allows ShowDetailPage
// to initialize with data that was prefetched before the component
// mounted, eliminating the skeleton flash on hard refresh.
const detailDataCache = new Map<string, ShowDetailResponse>();

/** Synchronously check if prefetched detail data is already resolved.
 *  Returns the data if available, null otherwise. Used by ShowDetailPage
 *  to initialize state without a loading flash when the prefetch
 *  completed before the component mounted. */
export function getCachedShowDetail(id: string | number): ShowDetailResponse | null {
  return detailDataCache.get(String(id)) ?? null;
}

/** Clear the cached detail data for a show. Called after ShowDetailPage
 *  consumes the initial prefetch — subsequent getShow/getEpisodes calls
 *  (e.g. loadShowData after a save) should fetch fresh data, not return
 *  stale cached data. Also clears the promise cache to prevent
 *  unbounded memory growth in long sessions. */
export function clearCachedShowDetail(id: string | number): void {
  const key = String(id);
  detailDataCache.delete(key);
  detailPrefetch.delete(key);
  // Also clear episode ETag/data caches to prevent unbounded memory
  // growth in long sessions where the user visits many show pages.
  episodeETags.delete(key);
  episodeDataCache.delete(key);
}

/** Prefetch show + episodes data. Used in main.tsx on direct
 *  navigation to /shows/:id. Reuses the pre-fired show + episodes
 *  fetches from the inline script in index.html. Resolved data is
 *  stored synchronously so ShowDetailPage can initialize with it
 *  (zero skeleton flash when the prefetch completes before mount).
 *  The individual show and episodes promises are stored separately
 *  so getShow can resolve without waiting for the slower episodes
 *  fetch (583B show vs 252KB episodes). */
export function prefetchShowDetail(id: string | number): void {
  const key = String(id);
  if (detailPrefetch.has(key)) return;
  // Reuse the pre-fired Responses from the inline script if available.
  const showPreFired = prefetch?.show && prefetch.detailId === key ? prefetch.show : undefined;
  const episodesPreFired =
    prefetch?.episodes && prefetch.detailId === key ? prefetch.episodes : undefined;

  const showPromise = showPreFired
    ? showPreFired.then((res) => {
        if (!res.ok) throw new Error("prefetch failed");
        return res.clone().json() as Promise<Show>;
      })
    : request<Show>("GET", "/mappings/" + key);
  const episodesPromise = episodesPreFired
    ? episodesPreFired.then(async (res) => {
        if (!res.ok) throw new Error("prefetch failed");
        const newEtag = res.headers.get("ETag");
        if (newEtag) episodeETags.set(key, newEtag);
        const data = (await res.clone().json()) as Episode[];
        if (newEtag) episodeDataCache.set(key, data);
        return data;
      })
    : requestEpisodesWithETag(key);

  detailPrefetch.set(key, { show: showPromise, episodes: episodesPromise });

  // When both resolve, populate the synchronous cache for getCachedShowDetail.
  Promise.all([showPromise, episodesPromise])
    .then(([show, episodes]) => {
      detailDataCache.set(key, { show, episodes });
    })
    .catch(() => {
      /* ignore — effect will re-fetch */
    });
}

export function createShow(data: MappingCreate): Promise<Show> {
  return request<Show>("POST", "/mappings", data);
}

export function updateShow(id: string | number, data: ShowUpdate): Promise<Show> {
  return request<Show>("PUT", "/mappings/" + id, data);
}

export function deleteShow(id: string | number): Promise<unknown> {
  return request<unknown>("DELETE", "/mappings/" + id);
}

export function autoMatch(sonarrSeriesId: string | number): Promise<Show> {
  return request<Show>("POST", "/mappings", { sonarr_series_id: sonarrSeriesId, auto_match: true });
}

export function confirmMatch(data: MappingCreate): Promise<Show> {
  return request<Show>("POST", "/mappings", data);
}

export function suggestAFL(seriesId: string | number): Promise<AFLMatch[]> {
  return request<AFLMatch[]>("GET", "/afl/suggest?series_id=" + encodeURIComponent(seriesId));
}

export function searchAFL(
  query: string,
  seriesId: string | number | null = null,
): Promise<AFLMatch[]> {
  const params = new URLSearchParams();
  params.set("q", query || "");
  if (seriesId != null) params.set("series_id", String(seriesId));
  return request<AFLMatch[]>("GET", "/afl/search?" + params.toString());
}

// In-memory cache for episode prefetches — when the user hovers a show
// card on the library page, we fire getEpisodes early and cache the
// promise. When they actually navigate to the detail page, the promise
// is already resolving (or resolved), so the episode list appears
// instantly. The cache entry is consumed once and then cleared, so it
// doesn't go stale.
const episodePrefetch = new Map<string, Promise<Episode[]>>();

// ETag cache for episode responses — the backend sends an ETag header
// based on episode count, monitored sum, and last-scraped timestamp.
// On repeat visits, we send If-None-Match and get a 304 instead of
// re-downloading the full payload (252KB for 1200+ episodes).
const episodeETags = new Map<string, string>();
const episodeDataCache = new Map<string, Episode[]>();

export function getEpisodes(showId: string | number): Promise<Episode[]> {
  const key = String(showId);
  const cached = episodePrefetch.get(key);
  if (cached) {
    episodePrefetch.delete(key);
    return cached;
  }
  // If prefetchShowDetail already resolved and cached the episodes,
  // return them directly — avoids double JSON parsing of the 252KB
  // episodes payload when both prefetchShowDetail and the effect
  // consume the same prefetched Response.
  const cachedDetail = detailDataCache.get(key);
  if (cachedDetail?.episodes) {
    return Promise.resolve(cachedDetail.episodes);
  }
  // If prefetchShowDetail has an in-flight episodes promise, use it
  // directly — avoids parsing the same Response body twice.
  const inflightDetail = detailPrefetch.get(key);
  if (inflightDetail) {
    return inflightDetail.episodes;
  }
  // Use the pre-fired Response from the inline script if available.
  // The inline script fires fetch("/api/mappings/:id/episodes") before
  // the JS bundle downloads, so the request is already in-flight.
  // Clone the Response in case prefetchShowDetail also reads it —
  // Response.json() can only be called once per Response.
  const preFired = prefetch?.episodes && prefetch.detailId === key ? prefetch.episodes : undefined;
  if (preFired) {
    return preFired.then(async (res) => {
      if (!res.ok) {
        // Fall through to ETag request
        return requestEpisodesWithETag(key);
      }
      const newEtag = res.headers.get("ETag");
      if (newEtag) episodeETags.set(key, newEtag);
      const data = (await res.clone().json()) as Episode[];
      if (newEtag) episodeDataCache.set(key, data);
      return data;
    });
  }
  return requestEpisodesWithETag(key);
}

async function requestEpisodesWithETag(key: string): Promise<Episode[]> {
  const headers: Record<string, string> = {};
  const etag = episodeETags.get(key);
  if (etag) headers["If-None-Match"] = etag;

  const res = await fetch(BASE + "/mappings/" + key + "/episodes", { headers });
  if (res.status === 304) {
    if (episodeDataCache.has(key)) {
      return episodeDataCache.get(key)!;
    }
    // 304 with no cached data — the ETag is stale (e.g. cache was cleared).
    // Drop the ETag and re-fetch without conditional headers.
    episodeETags.delete(key);
    return requestEpisodesWithETag(key);
  }
  if (!res.ok) {
    const err = (await res.json().catch(() => ({ error: res.statusText }))) as { error?: string };
    throw new Error(err.error || res.statusText);
  }
  const newEtag = res.headers.get("ETag");
  if (newEtag) {
    episodeETags.set(key, newEtag);
  }
  const data = (await res.json()) as Episode[];
  if (newEtag) {
    episodeDataCache.set(key, data);
  }
  return data;
}

/** Prefetch episodes for a show without consuming the result. The cached
 *  promise is picked up by getEpisodes() when the user navigates to the
 *  detail page. Safe to call multiple times — only the first call fires. */
export function prefetchEpisodes(showId: string | number): void {
  const key = String(showId);
  if (episodePrefetch.has(key)) return;
  episodePrefetch.set(key, requestEpisodesWithETag(key));
}

// --- Artwork (multi-provider, locally stored) ---

export function getArtworkProviders(): Promise<{ providers: ArtworkProvider[] }> {
  return request<{ providers: ArtworkProvider[] }>("GET", "/artwork/providers");
}

export function getArtworkPosterUrl(mappingId: string | number): string {
  return `${BASE}/artwork/${mappingId}/poster`;
}

export function getArtworkBannerUrl(mappingId: string | number): string {
  return `${BASE}/artwork/${mappingId}/banner`;
}

export function getArtworkLogoUrl(mappingId: string | number): string {
  return `${BASE}/artwork/${mappingId}/logo`;
}

export function refreshArtwork(
  showId: string | number,
  source?: string,
): Promise<ArtworkRefreshResult> {
  const query = source ? `?source=${encodeURIComponent(source)}` : "";
  return request<ArtworkRefreshResult>("POST", `/mappings/${showId}/artwork/refresh${query}`);
}

export function setArtworkSource(
  showId: string | number,
  source: string,
): Promise<{ status: string }> {
  return request<{ status: string }>("PUT", `/mappings/${showId}/artwork/source`, { source });
}

export function refreshAllArtwork(): Promise<{ status: string }> {
  return request<{ status: string }>("POST", "/artwork/refresh-all");
}

export function getArtworkRefreshStatus(): Promise<{ refreshing: boolean }> {
  return request<{ refreshing: boolean }>("GET", "/artwork/refresh-status");
}

export function setEpisodeMonitor(
  showId: string | number,
  epId: string | number,
  monitored: boolean,
): Promise<{ monitored: boolean }> {
  return request<{ monitored: boolean }>(
    "PUT",
    "/mappings/" + showId + "/episodes/" + epId + "/monitor",
    {
      monitored,
    },
  );
}

export function triggerSync(): Promise<unknown> {
  return request<unknown>("POST", "/sync");
}

export function getSyncLogs(limit: number = 12, offset: number = 0): Promise<SyncLogResponse> {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  params.set("offset", String(offset));
  return request<SyncLogResponse>("GET", "/logs?" + params.toString());
}

export function getSettings(): Promise<Settings> {
  return request<Settings>("GET", "/settings");
}

export function updateSettings(data: Settings | Partial<Settings>): Promise<{ status: string }> {
  return request<{ status: string }>("PUT", "/settings", data);
}

export function refreshAflCache(): Promise<unknown> {
  return request<unknown>("POST", "/settings/afl-cache/refresh");
}

export function testNtfyConnection(data: {
  server_url?: string;
  topic?: string;
  token?: string;
}): Promise<TestConnectionResult> {
  return request<TestConnectionResult>("POST", "/ntfy/test", data);
}

/** Safely extract a human-readable message from an unknown catch value. */
export function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return (
      err.message
        // Strip Go-style prefixes like 'Post "url": dial tcp: ...'
        .replace(/^(?:Post|Get|Put|Delete)\s+"[^"]+":\s*/i, "")
        // Trim 'dial tcp: lookup hostname: no such host' → 'hostname could not be reached'
        .replace(/^dial tcp:\s*lookup\s+\S+:\s*no such host/i, (m, _g) => {
          const host = m.match(/lookup\s+(\S+)/)?.[1] ?? "server";
          return `${host} could not be reached`;
        })
        // 'connection refused' → friendlier
        .replace(/^dial tcp.*: connection refused/i, "connection was refused by the server")
        // 'i/o timeout' → friendlier
        .replace(/^dial tcp.*: i\/o timeout/i, "connection timed out")
        // Collapse 'context deadline exceeded (client timeout exceeded while awaiting headers)'
        .replace(/^context deadline exceeded.*$/i, "request timed out")
    );
  }
  return String(err);
}
