const BASE = '/api';

async function request(method, path, body = null) {
  const opts = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body) opts.body = JSON.stringify(body);

  const res = await fetch(BASE + path, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export function getStatus() {
  return request('GET', '/health');
}

export function getVersion() {
  return request('GET', '/version');
}

export function getStats() {
  return request('GET', '/stats');
}

export function getSonarrSeries() {
  return request('GET', '/sonarr/series');
}

export function testSonarrConnection(url, apiKey) {
  return request('POST', '/sonarr/test', { url, api_key: apiKey });
}

export function getShows() {
  return request('GET', '/mappings');
}

export function getShow(id) {
  return request('GET', '/mappings/' + id);
}

export function createShow(data) {
  return request('POST', '/mappings', data);
}

export function updateShow(id, data) {
  return request('PUT', '/mappings/' + id, data);
}

export function deleteShow(id) {
  return request('DELETE', '/mappings/' + id);
}

export function autoMatch(sonarrSeriesId) {
  return request('POST', '/mappings', { sonarr_series_id: sonarrSeriesId, auto_match: true });
}

export function confirmMatch(data) {
  return request('POST', '/mappings', data);
}

export function suggestAFL(seriesId) {
  return request('GET', '/afl/suggest?series_id=' + encodeURIComponent(seriesId));
}

export function searchAFL(query, seriesId = null) {
  const params = new URLSearchParams();
  params.set('q', query || '');
  if (seriesId != null) params.set('series_id', String(seriesId));
  return request('GET', '/afl/search?' + params.toString());
}

export function getEpisodes(showId) {
  return request('GET', '/mappings/' + showId + '/episodes');
}

export function getSonarrEpisodes(showId) {
  return request('GET', '/mappings/' + showId + '/episodes/sonarr');
}

export function syncEpisode(showId, epNum) {
  return request('POST', '/mappings/' + showId + '/episodes/' + epNum + '/sync');
}

export function setEpisodeMonitor(showId, epId, monitored) {
  return request('PUT', '/mappings/' + showId + '/episodes/' + epId + '/monitor', { monitored });
}

export function resyncAll(showId) {
  return request('POST', '/mappings/' + showId + '/resync');
}

export function triggerSync() {
  return request('POST', '/sync');
}

export function getSyncLogs(limit = 12, offset = 0) {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  return request('GET', '/logs?' + params.toString());
}

export function getSettings() {
  return request('GET', '/settings');
}

export function updateSettings(data) {
  return request('PUT', '/settings', data);
}

export function refreshAflCache() {
  return request('POST', '/settings/afl-cache/refresh');
}
