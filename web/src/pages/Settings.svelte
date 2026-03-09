<script>
  import { onMount } from 'svelte';
  import { settings, addNotification } from '../lib/stores.js';
  import { getSettings, updateSettings, testSonarrConnection, refreshAflCache } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import IconFloppyDiskRegular from 'phosphor-icons-svelte/IconFloppyDiskRegular.svelte';
  import IconPlugRegular from 'phosphor-icons-svelte/IconPlugRegular.svelte';
  import IconArrowsClockwiseRegular from 'phosphor-icons-svelte/IconArrowsClockwiseRegular.svelte';

  let loading = true;
  let saving = false;
  let testing = false;
  let refreshingCache = false;
  let testResult = null;

  let sonarrUrl = '';
  let sonarrApiKey = '';
  let syncInterval = '6h';
  let enableTagging = false;
  let fillerTagName = 'filler';
  let trackFillerDefault = false;
  let trackMixedDefault = true;
  let trackAnimeCanonDefault = true;
  let cacheRefreshedAt = null;
  let cacheCount = 0;

  const syncIntervalOptions = [
    { value: '1h', label: 'Every 1 hour' },
    { value: '3h', label: 'Every 3 hours' },
    { value: '6h', label: 'Every 6 hours' },
    { value: '12h', label: 'Every 12 hours' },
    { value: '24h', label: 'Every 24 hours' },
  ];

  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  onMount(async () => {
    await loadSettings();
  });

  async function loadSettings() {
    loading = true;
    try {
      const data = await getSettings();
      if (data && !data.error) {
        sonarrUrl = data.sonarr_url || '';
        sonarrApiKey = data.sonarr_api_key || '';
        syncInterval = data.sync_interval || '6h';
        enableTagging = data.enable_tagging || false;
        fillerTagName = data.filler_tag_name || 'filler';
        trackFillerDefault = actionToTracked(data.default_filler_action, false);
        trackMixedDefault = actionToTracked(data.default_mixed_canon_action, true);
        trackAnimeCanonDefault = actionToTracked(data.default_anime_canon_action, true);
        cacheRefreshedAt = data.afl_show_cache_refreshed_at || null;
        cacheCount = data.afl_show_cache_count || 0;
        settings.set(data);
      }
    } catch (err) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
    loading = false;
  }

  async function handleSave() {
    saving = true;
    try {
      await updateSettings({
        sonarr_url: sonarrUrl,
        sonarr_api_key: sonarrApiKey,
        default_filler_action: trackedToAction(trackFillerDefault),
        default_mixed_canon_action: trackedToAction(trackMixedDefault),
        default_anime_canon_action: trackedToAction(trackAnimeCanonDefault),
        sync_interval: syncInterval,
        enable_tagging: enableTagging,
        filler_tag_name: fillerTagName,
      });
      addNotification('Settings saved successfully', 'success');
      await loadSettings();
    } catch (err) {
      addNotification('Failed to save settings: ' + err.message, 'error');
    }
    saving = false;
  }

  async function handleTestConnection() {
    if (!sonarrUrl) {
      addNotification('Please enter a Sonarr URL first', 'warning');
      return;
    }
    testing = true;
    testResult = null;
    try {
      await testSonarrConnection(sonarrUrl, sonarrApiKey);
      testResult = { success: true, message: 'Connection successful' };
      addNotification('Sonarr connection test passed', 'success');
    } catch (err) {
      testResult = { success: false, message: err.message };
      addNotification('Sonarr connection test failed: ' + err.message, 'error');
    }
    testing = false;
  }

  async function handleRefreshCache() {
    refreshingCache = true;
    try {
      await refreshAflCache();
      addNotification('AFL show list refreshed', 'success');
      await loadSettings();
    } catch (err) {
      addNotification('Failed to refresh AFL cache: ' + err.message, 'error');
    }
    refreshingCache = false;
  }

  function formatCacheTime(value) {
    if (!value) return 'Never refreshed';
    return new Date(value).toLocaleString();
  }
</script>

<svelte:head>
  <title>Mizu - Settings</title>
</svelte:head>

<div class="settings-shell">
  <section class="page-heading row-heading">
    <div>
      <h1>Settings</h1>
      <p>Sonarr connection, sync cadence, AFL refresh, and default tracking rules.</p>
    </div>
    <button class="btn btn-primary" on:click={handleSave} disabled={saving}>
      <IconFloppyDiskRegular size={18} weight="regular" />
      <span>{saving ? 'Saving...' : 'Save settings'}</span>
    </button>
  </section>

  {#if loading}
    <Spinner center size="lg" message="Loading Mizu settings..." />
  {:else}
    <section class="settings-grid">
      <div class="card settings-panel">
        <div class="panel-head">
          <h3>Sonarr connection</h3>
          {#if testResult}<span class="test-result" class:error={!testResult.success}>{testResult.message}</span>{/if}
        </div>
        <div class="form-group">
          <label class="form-label" for="sonarr-url">Sonarr URL</label>
          <input id="sonarr-url" class="form-input" type="text" placeholder="http://localhost:8989" bind:value={sonarrUrl} />
          <div class="form-hint">Full URL to your Sonarr instance.</div>
        </div>
        <div class="form-group">
          <label class="form-label" for="sonarr-key">API key</label>
          <input id="sonarr-key" class="form-input" type="password" placeholder="Your Sonarr API key" bind:value={sonarrApiKey} />
          <div class="form-hint">Found in Sonarr under Settings → General → Security.</div>
        </div>
        <button class="btn btn-secondary" on:click={handleTestConnection} disabled={testing}>
          <IconPlugRegular size={18} weight="regular" />
          <span>{testing ? 'Testing...' : 'Test connection'}</span>
        </button>
      </div>

      <div class="card settings-panel">
        <div class="panel-head"><h3>Sync behavior</h3></div>
        <div class="form-group">
          <label class="form-label" for="sync-interval">Sync interval</label>
          <select id="sync-interval" class="form-select" bind:value={syncInterval}>
            {#each syncIntervalOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
          <div class="form-hint">How often Mizu syncs filler data from AnimeFillerList.</div>
        </div>
        <div class="cache-status-card">
          <div><span>Show list refresh</span><strong>Monthly automatic refresh</strong></div>
          <div><span>Last refreshed</span><strong>{formatCacheTime(cacheRefreshedAt)}</strong></div>
          <div><span>Cached shows</span><strong>{cacheCount}</strong></div>
        </div>
        <button class="btn btn-secondary" on:click={handleRefreshCache} disabled={refreshingCache}>
          <IconArrowsClockwiseRegular size={18} weight="regular" />
          <span>{refreshingCache ? 'Refreshing...' : 'Refresh AFL show list'}</span>
        </button>
      </div>
    </section>

    <section class="settings-grid secondary">
      <div class="card settings-panel">
        <div class="panel-head"><h3>Default tracking rules</h3></div>
        <div class="check-list">
          <label class="check-card">
            <input type="checkbox" bind:checked={trackFillerDefault} />
            <div><strong>Track filler</strong><small>New mappings will monitor filler episodes when enabled.</small></div>
          </label>
          <label class="check-card">
            <input type="checkbox" bind:checked={trackMixedDefault} />
            <div><strong>Track mixed canon</strong><small>New mappings will monitor mixed canon episodes when enabled.</small></div>
          </label>
          <label class="check-card">
            <input type="checkbox" bind:checked={trackAnimeCanonDefault} />
            <div><strong>Track anime canon</strong><small>New mappings will monitor anime canon episodes when enabled.</small></div>
          </label>
        </div>
      </div>

      <div class="card settings-panel">
        <div class="panel-head"><h3>Tagging</h3></div>
        <label class="check-card">
          <input type="checkbox" bind:checked={enableTagging} />
          <div><strong>Enable filler tagging</strong><small>Add a tag to Sonarr series that contain filler-related episodes.</small></div>
        </label>
        {#if enableTagging}
          <div class="form-group" style="margin-top:16px">
            <label class="form-label" for="tag-name">Tag name</label>
            <input id="tag-name" class="form-input" type="text" placeholder="filler" bind:value={fillerTagName} />
            <div class="form-hint">The tag label to apply in Sonarr.</div>
          </div>
        {/if}
      </div>
    </section>
  {/if}
</div>

<style>
  .settings-shell {
    display: grid;
    gap: 18px;
    max-width: 1160px;
    margin: 0 auto;
  }

  .row-heading {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 16px;
  }

  .row-heading h1 {
    margin-bottom: 4px;
    font-size: clamp(1.75rem, 2vw, 2.05rem);
  }

  .row-heading p {
    color: var(--text-secondary);
  }

  .row-heading .btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .settings-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
  }

  .settings-panel {
    padding: 18px;
  }

  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 14px;
  }

  .test-result {
    color: var(--accent);
    font-size: 12px;
  }

  .test-result.error {
    color: var(--danger);
  }

  .settings-panel .btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .cache-status-card {
    display: grid;
    gap: 10px;
    margin-bottom: 16px;
    padding: 14px;
    border-radius: 18px;
    border: 1px solid rgba(118, 167, 203, 0.14);
    background: rgba(10, 17, 29, 0.7);
  }

  .cache-status-card span {
    display: block;
    margin-bottom: 4px;
    color: var(--text-muted);
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .cache-status-card strong {
    font-size: 13px;
  }

  .check-list {
    display: grid;
    gap: 10px;
  }

  .check-card {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 12px;
    align-items: start;
    padding: 12px 14px;
    border-radius: 16px;
    border: 1px solid rgba(149, 202, 234, 0.12);
    background: rgba(255, 255, 255, 0.03);
    cursor: pointer;
  }

  .check-card strong {
    display: block;
    margin-bottom: 4px;
    font-family: 'Figtree', sans-serif;
  }

  .check-card small {
    color: var(--text-secondary);
    font-size: 12px;
  }

  @media (max-width: 900px) {
    .row-heading {
      flex-direction: column;
      align-items: flex-start;
    }

    .settings-grid {
      grid-template-columns: 1fr;
    }
  }
</style>




