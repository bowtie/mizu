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

async function request<T>(method: string, path: string, body: unknown = null): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { "Content-Type": "application/json" },
  };
  if (body) opts.body = JSON.stringify(body);

  const res = await fetch(BASE + path, opts);
  if (!res.ok) {
    const err = (await res.json().catch(() => ({ error: res.statusText }))) as { error?: string };
    throw new Error(err.error || res.statusText);
  }
  return res.json() as Promise<T>;
}

// In-memory cache for Sonarr series by ID — survives navigations within
// the session. The backend also caches in SQLite for 24h, but this avoids
// a network round-trip when revisiting a show in the same session.
const seriesCache = new Map<number, SonarrSeries>();

export function getSonarrSeriesById(seriesId: number): Promise<SonarrSeries> {
  const cached = seriesCache.get(seriesId);
  if (cached) return Promise.resolve(cached);
  return request<SonarrSeries>("GET", "/sonarr/series/" + seriesId).then((series) => {
    seriesCache.set(seriesId, series);
    return series;
  });
}

export function getSonarrSeries(matchableOnly: boolean = false): Promise<SonarrSeries[]> {
  const params = new URLSearchParams();
  if (matchableOnly) params.set("matchable_only", "true");
  const suffix = params.size > 0 ? "?" + params.toString() : "";
  return request<SonarrSeries[]>("GET", "/sonarr/series" + suffix);
}

export function lookupSonarrSeries(query: string): Promise<SonarrSeries[]> {
  const params = new URLSearchParams();
  params.set("q", query || "");
  return request<SonarrSeries[]>("GET", "/sonarr/lookup?" + params.toString());
}

export function addSonarrSeries(data: SonarrAddPayload): Promise<SonarrSeries> {
  return request<SonarrSeries>("POST", "/sonarr/series", data).then((series) => {
    // Cache the freshly created series so subsequent lookups are instant.
    seriesCache.set(series.id, series);
    return series;
  });
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

// In-memory cache for show prefetches — when the user hovers a show
// card, we fire getShow early and cache the promise. When they navigate
// to the detail page, the promise is already resolving (or resolved).
const showPrefetch = new Map<string, Promise<Show>>();

export function getShow(id: string | number): Promise<Show> {
  const key = String(id);
  const cached = showPrefetch.get(key);
  if (cached) {
    showPrefetch.delete(key);
    return cached;
  }
  return request<Show>("GET", "/mappings/" + id);
}

/** Prefetch show data without consuming the result. The cached promise
 *  is picked up by getShow() when the user navigates to the detail page. */
export function prefetchShow(id: string | number): void {
  const key = String(id);
  if (showPrefetch.has(key)) return;
  showPrefetch.set(key, request<Show>("GET", "/mappings/" + id));
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
  return requestEpisodesWithETag(key);
}

async function requestEpisodesWithETag(key: string): Promise<Episode[]> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const etag = episodeETags.get(key);
  if (etag) headers["If-None-Match"] = etag;

  const res = await fetch(BASE + "/mappings/" + key + "/episodes", { headers });
  if (res.status === 304 && episodeDataCache.has(key)) {
    return episodeDataCache.get(key)!;
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
  return request<ArtworkRefreshResult>(
    "POST",
    `/mappings/${showId}/artwork/refresh${query}`,
  );
}

export function setArtworkSource(
  showId: string | number,
  source: string,
): Promise<{ status: string }> {
  return request<{ status: string }>(
    "PUT",
    `/mappings/${showId}/artwork/source`,
    { source },
  );
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
  return request<{ monitored: boolean }>("PUT", "/mappings/" + showId + "/episodes/" + epId + "/monitor", {
    monitored,
  });
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
    return err.message
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
      .replace(/^context deadline exceeded.*$/i, "request timed out");
  }
  return String(err);
}
