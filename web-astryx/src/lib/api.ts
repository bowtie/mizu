import ky, { HTTPError } from "ky";
import type {
  AFLMatch,
  ArtworkOptions,
  ArtworkProvider,
  ArtworkRefreshResult,
  ArtworkSelectPayload,
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
    afterResponse: [
      ({ response }) => {
        // Session expired or revoked — bounce to the login page.
        // Auth endpoints are exempt so the login flow itself never loops.
        if (
          response.status === 401 &&
          !response.url.includes("/auth/") &&
          location.pathname !== "/login"
        ) {
          const from = encodeURIComponent(location.pathname + location.search);
          location.assign(`/login?from=${from}`);
        }
      },
    ],
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
  // without waiting for the 252KB episodes fetch. If the prefetch
  // fails (e.g. a 401 on a hard refresh before login), fall back to
  // a fresh request instead of handing out a permanently rejected
  // promise.
  const inflightDetail = detailPrefetch.get(key);
  if (inflightDetail) {
    return inflightDetail.show.catch(() => request<Show>("GET", "/mappings/" + key));
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
  // Bump the fetch generation so a still-in-flight episode fetch (e.g. the
  // prefetch fired before this clear) can't resurrect the caches it just wiped:
  // its captured gen no longer matches, while a fresh fetch bumps to an even
  // higher value. (Deleting would let the next fetch re-bump from 0 and collide
  // with a stale fetch's captured gen.)
  episodeFetchGen.set(key, (episodeFetchGen.get(key) ?? 0) + 1);
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
  let episodesPromise: Promise<Episode[]>;
  if (episodesPreFired) {
    // Capture the generation up front so the async write-back can verify it
    // is still the newest fetch for the key when the response resolves.
    const gen = (episodeFetchGen.get(key) ?? 0) + 1;
    episodeFetchGen.set(key, gen);
    episodesPromise = episodesPreFired.then(async (res) => {
      if (!res.ok) throw new Error("prefetch failed");
      const newEtag = res.headers.get("ETag");
      const data = (await res.clone().json()) as Episode[];
      // Write ETag and data atomically (same synchronous block), and only
      // if this fetch is still the newest for the key — a slower older
      // prefetch must not clobber a newer fetch's pair.
      if (newEtag && episodeFetchGen.get(key) === gen) {
        episodeETags.set(key, newEtag);
        episodeDataCache.set(key, data);
        pruneEpisodeCaches();
      }
      return data;
    });
  } else {
    episodesPromise = requestEpisodesWithETag(key);
  }

  // Retained per-key entry — also acts as a supersede marker below so a stale
  // in-flight prefetch can't write back to the cache after the page has moved on.
  const entry: DetailPrefetchEntry = { show: showPromise, episodes: episodesPromise };
  detailPrefetch.set(key, entry);

  // Evict on failure: a prefetch that rejects (401 on a hard refresh
  // before login, transient network error) must not stay in the map —
  // otherwise getShow/getEpisodes would hand out the same rejected
  // promise forever and the detail page would wedge until a full
  // reload. The handlers swallow the rejection so these side-effect
  // chains themselves never become unhandled rejections; consumers of
  // the entry's promises get the real error and fall back to fresh
  // requests (see getShow/getEpisodes).
  const evictIfCurrent = () => {
    if (detailPrefetch.get(key) === entry) detailPrefetch.delete(key);
  };
  showPromise.catch(evictIfCurrent);
  episodesPromise.catch(evictIfCurrent);

  // When both resolve, populate the synchronous cache for getCachedShowDetail.
  // Only write if this prefetch is still the current entry for the key: the
  // detail page clears these caches (clearCachedShowDetail) once it has fired
  // its own fetches, and a stale prefetch resolving after that clear must not
  // resurrect a snapshot that loadShowData (after a save) would then serve via
  // getShow/getEpisodes as if it were fresh.
  //
  // The episode ETag/data caches written inside episodesPromise above are
  // guarded by the same per-key fetch generation as requestEpisodesWithETag,
  // and the ETag+data pair is written atomically in one synchronous block, so
  // a stale prefetch can't leave a mismatched {newer ETag, older data} pair.
  Promise.all([showPromise, episodesPromise])
    .then(([show, episodes]) => {
      if (detailPrefetch.get(key) !== entry) return;
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

// Per-key generation counter for episode fetches. Every episode fetch bumps
// the counter for its key when it starts; the write-back only lands if the
// captured generation is still current when the response resolves. Without
// this, two concurrent fetches for one key (e.g. a hover prefetch racing a
// post-save reload) can leave a mismatched {newer ETag, older data} pair: the
// newer ETag keeps validating on 304 while the older data is served forever.
const episodeFetchGen = new Map<string, number>();

// Bound the per-show episode caches: hovering a large library without
// visiting would otherwise retain a full episode array per show for the
// whole session. Maps keep insertion order, so evicting from the front is
// a simple FIFO cap — a 304 for an evicted key just re-fetches.
const EPISODE_CACHE_MAX = 50;

function pruneEpisodeCaches(): void {
  for (const map of [episodeETags, episodeDataCache]) {
    while (map.size > EPISODE_CACHE_MAX) {
      const oldest = map.keys().next().value;
      if (oldest === undefined) break;
      map.delete(oldest);
    }
  }
}

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
  // directly — avoids parsing the same Response body twice. On prefetch
  // failure, fall back to a fresh ETag-aware request rather than
  // returning a permanently rejected promise.
  const inflightDetail = detailPrefetch.get(key);
  if (inflightDetail) {
    return inflightDetail.episodes.catch(() => requestEpisodesWithETag(key));
  }
  // Use the pre-fired Response from the inline script if available.
  // The inline script fires fetch("/api/mappings/:id/episodes") before
  // the JS bundle downloads, so the request is already in-flight.
  // Clone the Response in case prefetchShowDetail also reads it —
  // Response.json() can only be called once per Response.
  const preFired = prefetch?.episodes && prefetch.detailId === key ? prefetch.episodes : undefined;
  if (preFired) {
    // Same generation guard as requestEpisodesWithETag: capture the gen for
    // this write-back so a newer fetch for the key supersedes it, and write
    // the ETag+data pair atomically in one synchronous block.
    const gen = (episodeFetchGen.get(key) ?? 0) + 1;
    episodeFetchGen.set(key, gen);
    return preFired.then(async (res) => {
      if (!res.ok) {
        // Fall through to ETag request
        return requestEpisodesWithETag(key);
      }
      const newEtag = res.headers.get("ETag");
      const data = (await res.clone().json()) as Episode[];
      if (newEtag && episodeFetchGen.get(key) === gen) {
        episodeETags.set(key, newEtag);
        episodeDataCache.set(key, data);
        pruneEpisodeCaches();
      }
      return data;
    });
  }
  return requestEpisodesWithETag(key);
}

async function requestEpisodesWithETag(key: string): Promise<Episode[]> {
  // Capture this fetch's generation up front. The write-back below only lands
  // if no newer fetch started for the same key by the time the response
  // resolves — a slower older response must not clobber a newer pair.
  const gen = (episodeFetchGen.get(key) ?? 0) + 1;
  episodeFetchGen.set(key, gen);

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
  const data = (await res.json()) as Episode[];
  // Write the ETag and data atomically (same synchronous block), and only if
  // this fetch is still the newest for the key. Writing them split across the
  // JSON await is exactly what let an older response pair its stale data with
  // a newer ETag that the server then kept validating (304) forever.
  if (newEtag && episodeFetchGen.get(key) === gen) {
    episodeETags.set(key, newEtag);
    episodeDataCache.set(key, data);
    pruneEpisodeCaches();
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

export function getArtworkCandidates(
  showId: string | number,
  source: string,
): Promise<ArtworkOptions> {
  return request<ArtworkOptions>(
    "GET",
    `/mappings/${showId}/artwork/candidates?source=${encodeURIComponent(source)}`,
  );
}

export function selectArtwork(
  showId: string | number,
  payload: ArtworkSelectPayload,
): Promise<ArtworkRefreshResult> {
  return request<ArtworkRefreshResult>("PUT", `/mappings/${showId}/artwork/select`, payload);
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

export function refreshAflCache(): Promise<{ status: string }> {
  return request<{ status: string }>("POST", "/settings/afl-cache/refresh");
}

// ── Config backup ──

/** Downloads the full config (mappings + settings) as a JSON blob. */
export function exportConfig(): Promise<Blob> {
  return api("backup/export").blob();
}

/** Restores a config file previously produced by exportConfig. */
export function importConfig(data: unknown): Promise<{
  settings_imported: number;
  mappings_imported: number;
}> {
  return request("POST", "/backup/import", data);
}

export interface AflCacheRefreshStatus {
  refreshing: boolean;
  last_error?: string;
}

export function getAflCacheRefreshStatus(): Promise<AflCacheRefreshStatus> {
  return request<AflCacheRefreshStatus>("GET", "/settings/afl-cache/status");
}

export function testNtfyConnection(data: {
  server_url?: string;
  topic?: string;
  token?: string;
}): Promise<TestConnectionResult> {
  return request<TestConnectionResult>("POST", "/ntfy/test", data);
}

// ── Auth ──

export interface AuthStatus {
  configured: boolean;
  authenticated: boolean;
}

export function getAuthStatus(): Promise<AuthStatus> {
  return request<AuthStatus>("GET", "/auth/status");
}

export function login(password: string): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/login", { password });
}

export function setupPassword(password: string): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/setup", { password });
}

export function logout(): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/logout").then((res) => {
    // Evict the service worker's stale-while-revalidate API cache so
    // library data fetched under this session can't be served to the
    // next one. Best-effort — the SW may not be registered or supported.
    if (typeof caches !== "undefined") {
      caches
        .keys()
        .then((keys) =>
          Promise.all(keys.filter((k) => k.endsWith("-api")).map((k) => caches.delete(k))),
        )
        .catch(() => {
          /* non-fatal */
        });
    }
    return res;
  });
}

export function changePassword(
  currentPassword: string,
  newPassword: string,
): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/change-password", {
    current_password: currentPassword,
    new_password: newPassword,
  });
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
