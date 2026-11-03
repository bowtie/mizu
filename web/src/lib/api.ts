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
import { rememberReturnPath } from "./redirect";

const BASE = "/api";

/** Drops the service worker's offline copy of API responses. Called
 *  whenever a session ends (sign-out, expiry, password change) so library
 *  data fetched under it can't be served to whoever uses the browser
 *  next. Best-effort: the SW may not be registered or supported. */
function clearOfflineApiCache(): void {
  if (typeof caches === "undefined") return;
  caches
    .keys()
    .then((keys) =>
      Promise.all(keys.filter((k) => k.endsWith("-api")).map((k) => caches.delete(k))),
    )
    .catch(() => {
      /* non-fatal */
    });
}

// ky instance — handles JSON serialization and error parsing. Caching is
// TanStack Query's job (see queries.ts); this module only talks HTTP.
const api = ky.create({
  prefix: BASE + "/",
  headers: { "Content-Type": "application/json" },
  retry: 0, // TanStack Query handles retries at a higher level
  hooks: {
    afterResponse: [
      ({ response }) => {
        // Session expired or revoked: forget the offline copy of its data
        // and bounce to the login page. Auth endpoints are exempt from the
        // redirect so the login flow itself never loops.
        if (response.status === 401) clearOfflineApiCache();
        if (
          response.status === 401 &&
          !response.url.includes("/auth/") &&
          location.pathname !== "/login"
        ) {
          rememberReturnPath(location.pathname + location.search);
          location.assign("/login");
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
/** Downloads a config backup. API keys and tokens are left out unless
 *  `includeSecrets` is set, which needs the dashboard password (not asked
 *  for when a sign-in proxy handles authentication or sign-in is off). */
export function exportConfig(includeSecrets = false, password = ""): Promise<Blob> {
  return api("backup/export", {
    method: "POST",
    json: { include_secrets: includeSecrets, password },
  }).blob();
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

/** How Mizu decides who may use it: its own password, a sign-in proxy in
 *  front of it, or no sign-in at all. */
export type AuthMode = "password" | "external" | "open";

export interface AuthStatus {
  /** False only on a fresh install, before the welcome flow picks sign-in. */
  configured: boolean;
  authenticated: boolean;
  mode: AuthMode;
}

export interface AccessSettings {
  mode: AuthMode;
  trusted_proxies: string;
  /** The address this browser's requests reach Mizu from: the proxy's,
   *  when `forwarded`. */
  client_address: string;
  via_trusted_proxy: boolean;
  /** The request carried proxy headers, so client_address is a proxy. */
  forwarded: boolean;
  /** The hostname this browser uses; sign-in off keeps answering to it. */
  host: string;
  /** Hostnames Mizu answers to with sign-in off, besides IPs and localhost. */
  allowed_hosts: string;
  /** Whether a Mizu password exists; without one, switching to password
   *  sign-in takes a new password. */
  password_set: boolean;
}

export function getAccessSettings(): Promise<AccessSettings> {
  return request<AccessSettings>("GET", "/auth/access");
}

export function updateAccessSettings(data: {
  mode: AuthMode;
  trusted_proxies: string;
  allowed_hosts: string;
  current_password: string;
  new_password: string;
}): Promise<AccessSettings & { status: AuthStatus }> {
  return request<AccessSettings & { status: AuthStatus }>("PUT", "/auth/access", data);
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

/** First-run setup without a password: turns sign-in off. */
export function setupOpen(): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/setup", { open: true });
}

export function logout(): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>("POST", "/auth/logout").then((res) => {
    clearOfflineApiCache();
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
  }).then((res) => {
    // Every session was revoked server-side; drop their offline data too.
    clearOfflineApiCache();
    return res;
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

export interface WebhookSettings {
  token: string;
}

export function getWebhookSettings(): Promise<WebhookSettings> {
  return request<WebhookSettings>("GET", "/settings/webhook");
}

export function regenerateWebhookToken(): Promise<WebhookSettings> {
  return request<WebhookSettings>("POST", "/settings/webhook/regenerate");
}

export interface FillerFile {
  episode_file_id: number;
  path: string;
  size: number;
  episodes: { sonarr_episode_id: number; episode_num: number; title: string; type: string }[];
  /** One of its episodes is monitored anyway, usually to keep it. */
  monitored: boolean;
}

export function getFillerFiles(showId: string | number): Promise<{ files: FillerFile[] }> {
  return request<{ files: FillerFile[] }>("GET", "/mappings/" + showId + "/filler-files");
}

export function deleteFillerFiles(
  showId: string | number,
  episodeFileIds: number[],
): Promise<{ deleted: number; freed_bytes: number; error?: string }> {
  return request("POST", "/mappings/" + showId + "/filler-files/delete", {
    episode_file_ids: episodeFileIds,
  });
}
