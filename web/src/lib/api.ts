import type {
  AFLMatch,
  Artwork,
  Episode,
  MappingCreate,
  Settings,
  Show,
  ShowUpdate,
  SonarrAddOptions,
  SonarrAddPayload,
  SonarrSeries,
  StatsResponse,
  StatusResponse,
  StoredApiKeyResponse,
  SyncLogResponse,
  TestConnectionResult,
  VersionResponse,
} from './types.ts';

const BASE = '/api';

async function request<T>(method: string, path: string, body: unknown = null): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body) opts.body = JSON.stringify(body);

  const res = await fetch(BASE + path, opts);
  if (!res.ok) {
    const err = (await res.json().catch(() => ({ error: res.statusText }))) as { error?: string };
    throw new Error(err.error || res.statusText);
  }
  return res.json() as Promise<T>;
}

export function getStatus(): Promise<StatusResponse> {
  return request<StatusResponse>('GET', '/health');
}

export function getVersion(): Promise<VersionResponse> {
  return request<VersionResponse>('GET', '/version');
}

export function getStats(): Promise<StatsResponse> {
  return request<StatsResponse>('GET', '/stats');
}

export function getSonarrSeries(matchableOnly: boolean = false): Promise<SonarrSeries[]> {
  const params = new URLSearchParams();
  if (matchableOnly) params.set('matchable_only', 'true');
  const suffix = params.size > 0 ? '?' + params.toString() : '';
  return request<SonarrSeries[]>('GET', '/sonarr/series' + suffix);
}

export function lookupSonarrSeries(query: string): Promise<SonarrSeries[]> {
  const params = new URLSearchParams();
  params.set('q', query || '');
  return request<SonarrSeries[]>('GET', '/sonarr/lookup?' + params.toString());
}

export function addSonarrSeries(data: SonarrAddPayload): Promise<SonarrSeries> {
  return request<SonarrSeries>('POST', '/sonarr/series', data);
}

export function getSonarrAddOptions(): Promise<SonarrAddOptions> {
  return request<SonarrAddOptions>('GET', '/sonarr/add-options');
}

export function markManualMatchPending(seriesId: string | number): Promise<unknown> {
  return request<unknown>('POST', '/sonarr/manual-match-pending', { sonarr_series_id: seriesId });
}

export function testSonarrConnection(url: string, apiKey: string): Promise<TestConnectionResult> {
  return request<TestConnectionResult>('POST', '/sonarr/test', { url, api_key: apiKey });
}

export function getShows(): Promise<Show[]> {
  return request<Show[]>('GET', '/mappings');
}

export function getShow(id: string | number): Promise<Show> {
  return request<Show>('GET', '/mappings/' + id);
}

export function createShow(data: MappingCreate): Promise<Show> {
  return request<Show>('POST', '/mappings', data);
}

export function updateShow(id: string | number, data: ShowUpdate): Promise<Show> {
  return request<Show>('PUT', '/mappings/' + id, data);
}

export function deleteShow(id: string | number): Promise<unknown> {
  return request<unknown>('DELETE', '/mappings/' + id);
}

export function autoMatch(sonarrSeriesId: string | number): Promise<Show> {
  return request<Show>('POST', '/mappings', { sonarr_series_id: sonarrSeriesId, auto_match: true });
}

export function confirmMatch(data: MappingCreate): Promise<Show> {
  return request<Show>('POST', '/mappings', data);
}

export function suggestAFL(seriesId: string | number): Promise<AFLMatch[]> {
  return request<AFLMatch[]>('GET', '/afl/suggest?series_id=' + encodeURIComponent(seriesId));
}

export function searchAFL(query: string, seriesId: string | number | null = null): Promise<AFLMatch[]> {
  const params = new URLSearchParams();
  params.set('q', query || '');
  if (seriesId != null) params.set('series_id', String(seriesId));
  return request<AFLMatch[]>('GET', '/afl/search?' + params.toString());
}

export function getEpisodes(showId: string | number): Promise<Episode[]> {
  return request<Episode[]>('GET', '/mappings/' + showId + '/episodes');
}

export function getAniListArtwork(showId: string | number): Promise<Artwork> {
  return request<Artwork>('GET', '/mappings/' + showId + '/artwork');
}

export function getSonarrEpisodes(showId: string | number): Promise<Episode[]> {
  return request<Episode[]>('GET', '/mappings/' + showId + '/episodes/sonarr');
}

export function syncEpisode(showId: string | number, epNum: string | number): Promise<Episode> {
  return request<Episode>('POST', '/mappings/' + showId + '/episodes/' + epNum + '/sync');
}

export function setEpisodeMonitor(showId: string | number, epId: string | number, monitored: boolean): Promise<Episode> {
  return request<Episode>('PUT', '/mappings/' + showId + '/episodes/' + epId + '/monitor', { monitored });
}

export function resyncAll(showId: string | number): Promise<Show> {
  return request<Show>('POST', '/mappings/' + showId + '/resync');
}

export function triggerSync(): Promise<unknown> {
  return request<unknown>('POST', '/sync');
}

export function getSyncLogs(limit: number = 12, offset: number = 0): Promise<SyncLogResponse> {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  return request<SyncLogResponse>('GET', '/logs?' + params.toString());
}

export function getSettings(): Promise<Settings> {
  return request<Settings>('GET', '/settings');
}

export function getStoredSonarrApiKey(): Promise<StoredApiKeyResponse> {
  return request<StoredApiKeyResponse>('GET', '/settings/sonarr-api-key');
}

export function updateSettings(data: Settings | Partial<Settings>): Promise<Settings> {
  return request<Settings>('PUT', '/settings', data);
}

export function refreshAflCache(): Promise<unknown> {
  return request<unknown>('POST', '/settings/afl-cache/refresh');
}
