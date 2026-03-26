<script>
  import { onMount } from 'svelte';
  import { settings, addNotification } from '../lib/stores.js';
  import { getSettings, getStoredSonarrApiKey, updateSettings, testSonarrConnection, refreshAflCache } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import IconFloppyDiskRegular from 'phosphor-icons-svelte/IconFloppyDiskRegular.svelte';
  import IconPlugRegular from 'phosphor-icons-svelte/IconPlugRegular.svelte';
  import IconArrowsClockwiseRegular from 'phosphor-icons-svelte/IconArrowsClockwiseRegular.svelte';
  import IconCheckRegular from 'phosphor-icons-svelte/IconCheckRegular.svelte';
  import IconXRegular from 'phosphor-icons-svelte/IconXRegular.svelte';
  import IconEyeRegular from 'phosphor-icons-svelte/IconEyeRegular.svelte';
  import IconEyeSlashRegular from 'phosphor-icons-svelte/IconEyeSlashRegular.svelte';
  import IconCaretDownRegular from 'phosphor-icons-svelte/IconCaretDownRegular.svelte';

  let loading = true;
  let saving = false;
  let testing = false;
  let refreshingCache = false;
  let revealingKey = false;
  let testResult = null;
  let showAdvancedTagging = false;

  let sonarrUrl = '';
  let sonarrApiKey = '';
  let savedApiKeyMasked = '';
  let hasSavedApiKey = false;
  let apiKeyVisible = false;
  let apiKeyDirty = false;
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
        savedApiKeyMasked = data.sonarr_api_key_masked || '';
        hasSavedApiKey = !!data.sonarr_api_key_set;
        sonarrApiKey = savedApiKeyMasked;
        apiKeyVisible = false;
        apiKeyDirty = false;
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

  function handleApiKeyInput(event) {
    apiKeyDirty = true;
    sonarrApiKey = event.currentTarget.value;
  }

  async function toggleApiKeyVisibility() {
    if (apiKeyVisible) {
      apiKeyVisible = false;
      if (!apiKeyDirty && hasSavedApiKey) {
        sonarrApiKey = savedApiKeyMasked;
      }
      return;
    }

    if (!apiKeyDirty && hasSavedApiKey) {
      revealingKey = true;
      try {
        const data = await getStoredSonarrApiKey();
        sonarrApiKey = data.sonarr_api_key || '';
      } catch (err) {
        addNotification('Failed to reveal saved API key: ' + err.message, 'error');
        revealingKey = false;
        return;
      }
      revealingKey = false;
    }

    apiKeyVisible = true;
  }

  async function resolveApiKeyForTest() {
    if (apiKeyDirty || apiKeyVisible || !hasSavedApiKey) {
      return sonarrApiKey;
    }
    const data = await getStoredSonarrApiKey();
    return data.sonarr_api_key || '';
  }

  async function handleSave() {
    saving = true;
    try {
      const payload = {
        sonarr_url: sonarrUrl,
        default_filler_action: trackedToAction(trackFillerDefault),
        default_mixed_canon_action: trackedToAction(trackMixedDefault),
        default_anime_canon_action: trackedToAction(trackAnimeCanonDefault),
        sync_interval: syncInterval,
        enable_tagging: enableTagging,
        filler_tag_name: fillerTagName,
      };

      if (apiKeyDirty || !hasSavedApiKey) {
        payload.sonarr_api_key = sonarrApiKey;
      }

      await updateSettings(payload);
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
      const apiKey = await resolveApiKeyForTest();
      const result = await testSonarrConnection(sonarrUrl, apiKey);
      if (!result?.success) {
        throw new Error(result?.error || 'Connection failed');
      }
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
      <p>Connect Sonarr, choose sync timing, and set default rules.</p>
    </div>
    <button class="btn btn-primary" on:click={handleSave} disabled={saving}>
      <IconFloppyDiskRegular size={20} weight="regular" />
      <span>{saving ? 'Saving...' : 'Save settings'}</span>
    </button>
  </section>

  {#if loading}
    <Spinner center size="lg" message="Loading settings from the current..." />
  {:else}
    <section class="settings-grid">
      <div class="card settings-panel">
        <div class="panel-head">
          <div>
            <h3>Sonarr</h3>
            <p>Connection settings for your Sonarr instance.</p>
          </div>
          {#if testResult}<span class="test-result" class:error={!testResult.success}>{testResult.message}</span>{/if}
        </div>

        <div class="panel-body">
        <div class="form-group">
          <label class="form-label" for="sonarr-url">Sonarr URL</label>
          <input id="sonarr-url" class="form-input settings-field" type="text" placeholder="http://localhost:8989" bind:value={sonarrUrl} />
          <div class="form-hint">Full URL to your Sonarr instance.</div>
        </div>

        <div class="form-group">
          <div class="field-head">
            <label class="form-label" for="sonarr-key">API key</label>
            {#if hasSavedApiKey || apiKeyDirty}
              <button type="button" class="inline-toggle" on:click={toggleApiKeyVisibility} disabled={revealingKey}>
                {#if apiKeyVisible}
                  <IconEyeSlashRegular size={20} weight="regular" />
                  <span>Hide</span>
                {:else}
                  <IconEyeRegular size={20} weight="regular" />
                  <span>{revealingKey ? 'Loading...' : 'View'}</span>
                {/if}
              </button>
            {/if}
          </div>
          <input
            id="sonarr-key"
            class="form-input settings-field"
            type={apiKeyVisible ? 'text' : 'password'}
            value={sonarrApiKey}
            placeholder={hasSavedApiKey ? 'Saved API key' : 'Your Sonarr API key'}
            on:input={handleApiKeyInput}
          />
          <div class="form-hint">{hasSavedApiKey && !apiKeyDirty ? 'Saved key is masked by default.' : 'Found in Sonarr under Settings → General → Security.'}</div>
        </div>

        <button class="btn btn-secondary" on:click={handleTestConnection} disabled={testing}>
          <IconPlugRegular size={20} weight="regular" />
          <span>{testing ? 'Testing...' : 'Test connection'}</span>
        </button>
        </div>
      </div>

      <div class="card settings-panel">
        <div class="panel-head">
          <div>
            <h3>Sync</h3>
            <p>Choose how often Mizu syncs and refreshes the AnimeFillerList show list.</p>
          </div>
        </div>

        <div class="panel-body">
        <div class="form-group">
          <label class="form-label" for="sync-interval">Sync interval</label>
          <select id="sync-interval" class="form-select settings-field" bind:value={syncInterval}>
            {#each syncIntervalOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
          <div class="form-hint">How often Mizu checks Sonarr and AnimeFillerList for changes.</div>
        </div>

        <div class="cache-status-card">
          <div><span>Show list refresh</span><strong>Monthly automatic refresh</strong></div>
          <div><span>Last refreshed</span><strong>{formatCacheTime(cacheRefreshedAt)}</strong></div>
          <div><span>Cached shows</span><strong>{cacheCount}</strong></div>
        </div>

        <button class="btn btn-secondary" on:click={handleRefreshCache} disabled={refreshingCache}>
          <IconArrowsClockwiseRegular size={20} weight="regular" />
          <span>{refreshingCache ? 'Refreshing...' : 'Refresh show list'}</span>
        </button>
        </div>
      </div>
    </section>

    <section class="settings-grid secondary">
      <div class="card settings-panel">
        <div class="panel-head">
          <div>
            <h3>Defaults</h3>
            <p>Default tracking rules for newly added anime.</p>
          </div>
        </div>

        <div class="panel-body">
        <div class="rule-pill-row" role="group" aria-label="Default tracking rules">
          <label class="rule-pill rule-pill-anime" class:active={trackAnimeCanonDefault}>
            <input type="checkbox" bind:checked={trackAnimeCanonDefault} />
            {#if trackAnimeCanonDefault}<IconCheckRegular size={20} weight="bold" />{:else}<IconXRegular size={20} weight="bold" />{/if}
            <span>Anime canon</span>
          </label>
          <label class="rule-pill rule-pill-mixed" class:active={trackMixedDefault}>
            <input type="checkbox" bind:checked={trackMixedDefault} />
            {#if trackMixedDefault}<IconCheckRegular size={20} weight="bold" />{:else}<IconXRegular size={20} weight="bold" />{/if}
            <span>Mixed canon</span>
          </label>
          <label class="rule-pill rule-pill-filler" class:active={trackFillerDefault}>
            <input type="checkbox" bind:checked={trackFillerDefault} />
            {#if trackFillerDefault}<IconCheckRegular size={20} weight="bold" />{:else}<IconXRegular size={20} weight="bold" />{/if}
            <span>Filler</span>
          </label>
        </div>
        </div>
      </div>

      <div class="card settings-panel advanced-panel">
        <button class="advanced-toggle" type="button" on:click={() => (showAdvancedTagging = !showAdvancedTagging)}>
          <div>
            <h3>Advanced tagging</h3>
            <p>Add a Sonarr tag to anime with filler-related episodes.</p>
          </div>
          <span class:open={showAdvancedTagging}><IconCaretDownRegular size={20} weight="bold" /></span>
        </button>

        {#if showAdvancedTagging}
          <div class="advanced-content">
            <label class="check-card subdued-check-card">
              <input type="checkbox" bind:checked={enableTagging} />
              <div><strong>Enable Sonarr tagging</strong><small>Useful if you want to filter or spot filler-heavy anime inside Sonarr.</small></div>
            </label>
            {#if enableTagging}
              <div class="form-group advanced-form-group">
                <label class="form-label" for="tag-name">Tag name</label>
                <input id="tag-name" class="form-input settings-field" type="text" placeholder="filler" bind:value={fillerTagName} />
                <div class="form-hint">The tag label to apply in Sonarr.</div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </section>
  {/if}
</div>

<style>
  .settings-shell {
    position: relative;
    isolation: isolate;
    display: grid;
    gap: 20px;
    max-width: var(--content-max);
    margin: 0 auto;
  }

  .row-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 16px;
  }

  .row-heading h1 {
    font-size: 1.6rem;
    font-weight: 800;
    line-height: 1.1;
    letter-spacing: -0.02em;
  }

  .row-heading p {
    color: var(--text-muted);
    font-size: 13px;
    margin-top: 3px;
  }

  .row-heading .btn,
  .settings-panel .btn {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    flex-shrink: 0;
  }

  .settings-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    align-items: start;
  }

  .settings-panel {
    padding: 0;
    display: grid;
    gap: 0;
    background: var(--surface);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }

  .panel-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
    padding: 16px 18px 14px;
    background: var(--surface-elevated);
  }

  .panel-head h3 {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--text-primary);
  }

  .panel-head p {
    margin-top: 3px;
    color: var(--text-muted);
    font-size: 13px;
    line-height: 1.4;
  }

  .panel-body {
    padding: 16px 18px;
    display: grid;
    gap: 14px;
  }

  .test-result {
    color: var(--text-primary);
    font-size: 13px;
  }

  .test-result.error {
    color: var(--danger);
  }

  .field-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 7px;
  }

  .field-head .form-label {
    margin-bottom: 0;
  }

  .inline-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 600;
  }

  .settings-field {
    background: var(--surface-elevated);
    border-color: var(--border);
  }

  .settings-field:focus {
    border-color: var(--focus-border);
  }

  .cache-status-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 14px;
    border-radius: var(--radius);
    background: var(--surface-elevated);
  }

  .cache-status-card span {
    display: block;
    color: var(--text-muted);
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    margin-bottom: 2px;
  }

  .cache-status-card strong {
    font-size: 14px;
    color: var(--text-primary);
  }

  .rule-pill-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .rule-pill {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-height: 34px;
    padding: 0 13px;
    border-radius: 999px;
    background: var(--surface-elevated);
    color: var(--text-muted);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .rule-pill input {
    position: absolute;
    inset: 0;
    opacity: 0;
    pointer-events: none;
  }

  .rule-pill-anime.active,
  .rule-pill-mixed.active,
  .rule-pill-filler.active {
    background: var(--success-bg);
    color: var(--success);
  }

  .rule-pill-anime:not(.active),
  .rule-pill-mixed:not(.active),
  .rule-pill-filler:not(.active) {
    background: var(--danger-bg);
    color: var(--danger);
  }

  .advanced-panel {
    gap: 0;
  }

  .advanced-toggle {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    width: 100%;
    text-align: left;
    padding: 14px 18px;
  }

  .advanced-toggle p {
    margin-top: 3px;
    color: var(--text-muted);
    font-size: 13px;
  }

  .advanced-toggle span {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    transition: transform var(--transition);
    flex-shrink: 0;
  }

  .advanced-toggle span.open {
    transform: rotate(180deg);
  }

  .advanced-content {
    display: grid;
    gap: 12px;
    padding: 0 18px 16px;
    border-top: 1px solid var(--border);
    margin-top: 0;
    padding-top: 14px;
  }

  .check-card {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px 14px;
    border-radius: var(--radius);
    background: var(--surface-elevated);
    cursor: pointer;
    transition: border-color var(--transition), background var(--transition);
  }

  .check-card:has(input:checked) {
    background: rgba(26, 143, 255, 0.06);
  }

  .check-card input {
    margin-top: 3px;
    accent-color: var(--blue-base);
  }

  .check-card strong {
    display: block;
    margin-bottom: 3px;
    font-size: 14px;
    color: var(--text-primary);
  }

  .check-card small {
    display: block;
    color: var(--text-muted);
    font-size: 12px;
    line-height: 1.4;
  }

  .subdued-check-card {
    background: var(--surface);
  }

  .advanced-form-group {
    margin-bottom: 0;
  }

  @media (max-width: 900px) {
    .settings-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 720px) {
    .row-heading {
      align-items: stretch;
      flex-direction: column;
    }

    .row-heading .btn {
      width: 100%;
      justify-content: center;
    }

    .rule-pill-row {
      display: grid;
      grid-template-columns: 1fr;
    }
  }
</style>







