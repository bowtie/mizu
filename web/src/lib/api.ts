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

// ky instance — handles JSON serialization and error parsing. Caching is
// TanStack Query's job (see queries.ts); this module only talks HTTP.
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
        // ky has already read and parsed the body into `error.data` (the
        // response itself can't be read again). Surface the server's own
        // message; a non-JSON body leaves ky's default.
        if (error instanceof HTTPError) {
          const data = error.data as { error?: unknown } | undefined;
          if (typeof data?.error === "string" && data.error) {
            error.message = data.error;
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
  return request<SonarrSeries>("POST", "/sonarr/series", data);
}

export function getSonarrAddOptions(): Promise<SonarrAddOptions> {
  return request<SonarrAddOptions>("GET", "/sonarr/add-options");
}

export function markManualMatchPending(seriesId: string | number): Promise<unknown> {
  return request<unknown>("POST", "/sonarr/manual-match-pending", { sonarr_series_id: seriesId });
}

/** An empty `apiKey` tests with the key already saved on the server. */
export function testSonarrConnection(url: string, apiKey: string): Promise<TestConnectionResult> {
  return request<TestConnectionResult>("POST", "/sonarr/test", { url, api_key: apiKey });
}

export function getShows(): Promise<Show[]> {
  return request<Show[]>("GET", "/mappings");
}

export function getShow(id: string | number): Promise<Show> {
  return request<Show>("GET", "/mappings/" + id);
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

// The server tags episode lists with an ETag; the browser's HTTP cache
// revalidates it, so a repeat visit costs a 304, not the full list.
export function getEpisodes(showId: string | number): Promise<Episode[]> {
  return request<Episode[]>("GET", "/mappings/" + showId + "/episodes");
}

// --- Artwork (multi-provider, locally stored) ---

export function getArtworkProviders(): Promise<{ providers: ArtworkProvider[] }> {
  return request<{ providers: ArtworkProvider[] }>("GET", "/artwork/providers");
}

// The server lets browsers cache local artwork for a day, and the path for a
// show's poster/banner/logo never changes. Each URL therefore carries a
// version: a short hash of what the stored image is (its source URL), so a
// newly stored image gets a new URL while an unchanged one stays cached.
function artworkUrl(mappingId: string | number, kind: string, version?: string): string {
  const url = `${BASE}/artwork/${mappingId}/${kind}`;
  return version ? `${url}?v=${hashVersion(version)}` : url;
}

/** Short, stable, non-cryptographic hash (FNV-1a) for cache-busting. */
function hashVersion(value: string): string {
  let hash = 0x811c9dc5;
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0).toString(36);
}

/** `version` identifies the stored image — pass the show's
 *  `artwork_poster_url` (or path) so the URL changes when the image does. */
export function getArtworkPosterUrl(mappingId: string | number, version?: string): string {
  return artworkUrl(mappingId, "poster", version);
}

export function getArtworkBannerUrl(mappingId: string | number, version?: string): string {
  return artworkUrl(mappingId, "banner", version);
}

export function getArtworkLogoUrl(mappingId: string | number, version?: string): string {
  return artworkUrl(mappingId, "logo", version);
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

/** Stores an image the user chose for one slot of a show's artwork. */
export function uploadCustomArtwork(
  showId: string | number,
  slot: string,
  file: File,
): Promise<Show> {
  const form = new FormData();
  form.set("file", file);
  // The shared instance defaults to a JSON content type; a multipart body
  // needs the browser to set its own (with the boundary).
  return api(`mappings/${showId}/artwork/custom/${slot}`, {
    method: "POST",
    body: form,
    headers: { "Content-Type": undefined },
  }).json() as Promise<Show>;
}

/** Drops an uploaded image so a refresh can fill the slot again. */
export function removeCustomArtwork(showId: string | number, slot: string): Promise<Show> {
  return request<Show>("DELETE", `/mappings/${showId}/artwork/custom/${slot}`);
}

export function refreshAllArtwork(): Promise<{ status: string }> {
  return request<{ status: string }>("POST", "/artwork/refresh-all");
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
    // Evict the service worker's offline API cache so library data fetched
    // under this session can't be served to the next one. Best-effort —
    // the SW may not be registered or supported.
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

export function removePassword(): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/remove-password", {});
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
