<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { fade, slide } from 'svelte/transition';
  import { Switch } from '@skeletonlabs/skeleton-svelte';
  import { settings, addNotification } from '../../lib/stores.ts';
  import { getSettings, getStoredSonarrApiKey, refreshAflCache, testSonarrConnection, updateSettings } from '../../lib/api.ts';
  import type { Settings, StoredApiKeyResponse } from '$lib/types.ts';
  import { AlertTriangle, Check, Eye, EyeOff, LoaderCircle, Minus, Plug, Plus, RefreshCw, Save } from 'lucide-svelte';

  let loading = true;
  let saving = false;
  let testing = false;
  let refreshingCache = false;
  let revealingKey = false;
  let testPhase: 'idle' | 'connecting' | 'success' | 'error' = 'idle';
  let testErrorMessage = '';

  let sonarrUrl = '';
  let sonarrApiKey = '';
  let savedApiKeyMasked = '';
  let hasSavedApiKey = false;
  let apiKeyVisible = false;
  let apiKeyDirty = false;
  let syncInterval = '6h';
  let enableAniListArtwork = false;
  let trackFillerDefault = false;
  let trackMixedDefault = true;
  let trackAnimeCanonDefault = true;
  let cacheRefreshedAt: string | null = null;
  let cacheCount = 0;

  interface PristineSnapshot {
    sonarrUrl: string;
    sonarrApiKey: string;
    apiKeyDirty: boolean;
    syncInterval: string;
    enableAniListArtwork: boolean;
    trackFillerDefault: boolean;
    trackMixedDefault: boolean;
    trackAnimeCanonDefault: boolean;
  }

  let pristine: PristineSnapshot = {
    sonarrUrl: '',
    sonarrApiKey: '',
    apiKeyDirty: false,
    syncInterval: '6h',
    enableAniListArtwork: false,
    trackFillerDefault: false,
    trackMixedDefault: true,
    trackAnimeCanonDefault: true,
  };

  interface SyncIntervalOption {
    value: string;
    label: string;
  }

  const syncIntervalOptions: SyncIntervalOption[] = [
    { value: '1h', label: '1 hour' },
    { value: '3h', label: '3 hours' },
    { value: '6h', label: '6 hours' },
    { value: '12h', label: '12 hours' },
    { value: '24h', label: '24 hours' },
  ];

  $: syncIntervalIndex = syncIntervalOptions.findIndex((o) => o.value === syncInterval);
  function stepSyncInterval(dir: number) {
    const next = Math.min(Math.max(syncIntervalIndex + dir, 0), syncIntervalOptions.length - 1);
    syncInterval = syncIntervalOptions[next].value;
  }

  const actionToTracked = (action: string | undefined, fallback = true): boolean => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

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
        enableAniListArtwork = data.enable_anilist_artwork || false;
        trackFillerDefault = actionToTracked(data.default_filler_action, false);
        trackMixedDefault = actionToTracked(data.default_mixed_canon_action, true);
        trackAnimeCanonDefault = actionToTracked(data.default_anime_canon_action, true);
        cacheRefreshedAt = data.afl_show_cache_refreshed_at || null;
        cacheCount = data.afl_show_cache_count || 0;
        settings.set(data);
        pristine = {
          sonarrUrl,
          sonarrApiKey,
          apiKeyDirty,
          syncInterval,
          enableAniListArtwork,
          trackFillerDefault,
          trackMixedDefault,
          trackAnimeCanonDefault,
        };
      }
    } catch (err: any) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
    loading = false;
  }

  $: settingsDirty = !loading && (
    sonarrUrl !== pristine.sonarrUrl ||
    apiKeyDirty ||
    syncInterval !== pristine.syncInterval ||
    enableAniListArtwork !== pristine.enableAniListArtwork ||
    trackFillerDefault !== pristine.trackFillerDefault ||
    trackMixedDefault !== pristine.trackMixedDefault ||
    trackAnimeCanonDefault !== pristine.trackAnimeCanonDefault
  );

  $: urlValid = !sonarrUrl || /^https?:\/\/.+/.test(sonarrUrl);

  const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

  let prefersReducedMotion = false;
  onMount(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    prefersReducedMotion = mq.matches;
    const handler = (e: MediaQueryListEvent) => (prefersReducedMotion = e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  });
  const slideIn = (duration = 200): { duration: number } =>
    prefersReducedMotion ? { duration: 0 } : { duration };

  let resetTimer: ReturnType<typeof setTimeout> | null = null;
  function scheduleReset(delay = 1800) {
    clearTimeout(resetTimer as unknown as number);
    resetTimer = setTimeout(() => {
      testPhase = 'idle';
      testErrorMessage = '';
    }, delay);
  }
  function clearReset() {
    clearTimeout(resetTimer as unknown as number);
    resetTimer = null;
  }
  onDestroy(() => clearTimeout(resetTimer as unknown as number));

  function clearTestResult() {
    clearReset();
    if (testPhase === 'success' || testPhase === 'error') {
      testPhase = 'idle';
      testErrorMessage = '';
    }
  }

  function handleApiKeyInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    apiKeyDirty = true;
    sonarrApiKey = input.value;
    clearTestResult();
  }

  function handleUrlInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    sonarrUrl = input.value;
    clearTestResult();
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
      } catch (err: any) {
        addNotification('Failed to reveal saved API key: ' + err.message, 'error');
        revealingKey = false;
        return;
      }
      revealingKey = false;
    }

    apiKeyVisible = true;
  }

  async function resolveApiKeyForTest(): Promise<string> {
    if (apiKeyDirty || apiKeyVisible || !hasSavedApiKey) {
      return sonarrApiKey;
    }
    const data = await getStoredSonarrApiKey();
    return data.sonarr_api_key || '';
  }

  async function handleSave() {
    saving = true;
    try {
      const payload: Partial<Settings> = {
        sonarr_url: sonarrUrl,
        default_filler_action: trackedToAction(trackFillerDefault),
        default_mixed_canon_action: trackedToAction(trackMixedDefault),
        default_anime_canon_action: trackedToAction(trackAnimeCanonDefault),
        sync_interval: syncInterval,
        enable_anilist_artwork: enableAniListArtwork,
      };

      if (apiKeyDirty || !hasSavedApiKey) {
        payload.sonarr_api_key = sonarrApiKey;
      }

      await updateSettings(payload);
      addNotification('Settings saved', 'success');
      await loadSettings();
    } catch (err: any) {
      addNotification('Failed to save settings: ' + err.message, 'error');
    }
    saving = false;
  }

  async function handleTestConnection() {
    if (!sonarrUrl) {
      testErrorMessage = 'Enter a Sonarr URL first';
      testPhase = 'error';
      return;
    }
    if (!urlValid) {
      testErrorMessage = 'URL must start with http:// or https://';
      testPhase = 'error';
      return;
    }

    testing = true;
    testErrorMessage = '';
    clearReset();
    testPhase = 'connecting';

    const startTime = Date.now();
    const minDelay = 1100;

    try {
      const apiKey = await resolveApiKeyForTest();
      const result = await testSonarrConnection(sonarrUrl, apiKey);

      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);

      if (!result?.success) {
        throw new Error(result?.error || 'Connection failed');
      }

      testPhase = 'success';
      scheduleReset(2000);
    } catch (err: any) {
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);

      testErrorMessage = err.message;
      testPhase = 'error';
      scheduleReset(4000);
    }
    testing = false;
  }

  async function handleRefreshCache() {
    refreshingCache = true;
    try {
      await refreshAflCache();
      addNotification('AFL show list refreshed', 'success', 6000);
      await loadSettings();
    } catch (err: any) {
      addNotification('Failed to refresh AFL cache: ' + err.message, 'error');
    }
    refreshingCache = false;
  }

  function formatCacheTime(value: string | null | undefined): string {
    if (!value) return 'Never refreshed';
    return new Date(value).toLocaleString();
  }
</script>

<svelte:head>
  <title>Mizu - Settings</title>
</svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-surface-50">Settings</h1>
      <p class="mt-1 text-sm text-surface-400">Configure Sonarr connection, sync behavior, and defaults.</p>
    </div>
    <div class="flex items-center gap-3">
      {#if settingsDirty}
        <span class="badge preset-filled-warning-500 font-mono text-[10px] uppercase tracking-wider">Unsaved changes</span>
      {/if}
      <button class="btn btn-base preset-filled-primary-500" onclick={handleSave} disabled={saving || !settingsDirty}>
        {#if saving}
          <LoaderCircle size={16} class="animate-spin" />
        {:else}
          <Save size={16} />
        {/if}
        {saving ? 'Saving...' : 'Save settings'}
      </button>
    </div>
  </section>

  {#if loading}
    <div class="card preset-tonal-surface flex min-h-56 flex-col items-center justify-center gap-4 p-5 text-center">
      <LoaderCircle size={30} class="animate-spin text-primary-500" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-surface-50">Loading settings</h2>
        <p class="text-sm text-surface-400">Pulling current Sonarr and sync configuration.</p>
      </div>
    </div>
  {:else}
    <div class="space-y-6">
      <!-- Connection group -->
      <section class="card preset-tonal-surface border-surface-800">
        <div class="card-body p-5 gap-0">
          <h2 class="text-sm font-mono uppercase tracking-wider text-surface-400 mb-3">Connection</h2>

          <!-- Sonarr URL -->
          <div class="flex items-center gap-4 border-b border-surface-800 py-3.5">
            <div class="min-w-0 flex-1">
              <label for="sonarr-url" class="text-[13px] font-medium text-surface-50">Sonarr URL</label>
              <p class="mt-0.5 text-xs text-surface-400">Base URL of your Sonarr instance.</p>
              {#if sonarrUrl && !urlValid}
                <p class="mt-1 text-xs text-error-500">URL must start with http:// or https://</p>
              {/if}
            </div>
            <input
              id="sonarr-url"
              type="text"
              class="input field-base w-64 shrink-0 font-mono text-[13px]"
              placeholder="http://localhost:8989"
              value={sonarrUrl}
              oninput={handleUrlInput}
            />
          </div>

          <!-- API key -->
          <div class="flex items-center gap-4 border-b border-surface-800 py-3.5">
            <div class="min-w-0 flex-1">
              <label for="sonarr-key" class="text-[13px] font-medium text-surface-50">API key</label>
              <p class="mt-0.5 text-xs text-surface-400">Found in Sonarr under Settings, General.</p>
            </div>
            <div class="relative shrink-0">
              <input
                id="sonarr-key"
                type={apiKeyVisible ? 'text' : 'password'}
                class="input field-base w-64 font-mono text-[13px] {hasSavedApiKey || apiKeyDirty ? 'pr-10' : ''}"
                bind:value={sonarrApiKey}
                placeholder={hasSavedApiKey ? 'Saved API key' : 'Your Sonarr API key'}
                oninput={handleApiKeyInput}
              />
              {#if hasSavedApiKey || apiKeyDirty}
                <button
                  type="button"
                  onclick={toggleApiKeyVisibility}
                  disabled={revealingKey}
                  class="absolute inset-y-0 right-2 flex items-center text-surface-400 transition-colors hover:text-surface-50 disabled:opacity-50"
                  aria-label={apiKeyVisible ? 'Hide API key' : 'Reveal API key'}
                >
                  {#if apiKeyVisible}
                    {#key 'hide'}
                      <span class="flex items-center transition-opacity duration-100" in:fade={{ duration: 100 }}>
                        <EyeOff size={16} />
                      </span>
                    {/key}
                  {:else if revealingKey}
                    {#key 'loading'}
                      <span class="flex items-center transition-opacity duration-100" in:fade={{ duration: 100 }}>
                        <LoaderCircle size={16} class="animate-spin" />
                      </span>
                    {/key}
                  {:else}
                    {#key 'view'}
                      <span class="flex items-center transition-opacity duration-100" in:fade={{ duration: 100 }}>
                        <Eye size={16} />
                      </span>
                    {/key}
                  {/if}
                </button>
              {/if}
            </div>
          </div>

          <!-- Test connection -->
          <div class="flex items-center gap-4 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">Test connection</div>
              <p class="mt-0.5 text-xs text-surface-400">Check that Mizu can reach Sonarr with the URL and API key above.</p>
              {#if testPhase === 'error' && testErrorMessage}
                <div class="mt-2 card preset-filled-error-500 py-2 text-[13px]" transition:slide={slideIn(200)}>
                  {testErrorMessage}
                </div>
              {/if}
            </div>
            <button
              class="btn btn-base shrink-0 {testPhase === 'success' ? 'preset-filled-success-500' : testPhase === 'error' ? 'preset-filled-error-500' : 'preset-tonal'}"
              onclick={handleTestConnection}
              disabled={testing || testPhase === 'success' || !sonarrUrl || !urlValid}
              aria-busy={testing}
            >
              {#if testPhase === 'connecting'}
                <LoaderCircle size={16} class="animate-spin" />
                Testing
              {:else if testPhase === 'success'}
                <Check size={16} />
                Connected
              {:else if testPhase === 'error'}
                <AlertTriangle size={16} />
                Retry
              {:else}
                <Plug size={16} />
                Test
              {/if}
            </button>
          </div>
        </div>
      </section>

      <!-- Sync group -->
      <section class="card preset-tonal-surface border-surface-800">
        <div class="card-body p-5 gap-0">
          <h2 class="text-sm font-mono uppercase tracking-wider text-surface-400 mb-3">Sync</h2>

          <!-- Sync interval stepper -->
          <div class="flex items-center gap-4 border-b border-surface-800 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">Sync interval</div>
              <p class="mt-0.5 text-xs text-surface-400">How often Mizu checks for episode changes and updates Sonarr.</p>
            </div>
            <div class="flex shrink-0 items-center rounded-lg border border-surface-800">
              <button
                type="button"
                class="flex size-9 items-center justify-center rounded-l-lg text-surface-300 transition hover:bg-surface-800 hover:text-surface-50 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-surface-300"
                onclick={() => stepSyncInterval(-1)}
                disabled={syncIntervalIndex <= 0}
                aria-label="Decrease sync interval"
              >
                <Minus size={16} />
              </button>
              <div class="flex h-9 min-w-[5rem] items-center justify-center border-x border-surface-800 px-3 text-center font-mono text-xs tabular-nums text-surface-50">
                {syncIntervalOptions[syncIntervalIndex].label}
              </div>
              <button
                type="button"
                class="flex size-9 items-center justify-center rounded-r-lg text-surface-300 transition hover:bg-surface-800 hover:text-surface-50 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-surface-300"
                onclick={() => stepSyncInterval(1)}
                disabled={syncIntervalIndex >= syncIntervalOptions.length - 1}
                aria-label="Increase sync interval"
              >
                <Plus size={16} />
              </button>
            </div>
          </div>

          <!-- AniList artwork -->
          <div class="flex items-center gap-4 border-b border-surface-800 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">AniList artwork</div>
              <p class="mt-0.5 text-xs text-surface-400">Use AniList banners and posters instead of Sonarr images only.</p>
            </div>
            <Switch checked={enableAniListArtwork} onCheckedChange={(d) => (enableAniListArtwork = d.checked)}><Switch.Control><Switch.Thumb /></Switch.Control><Switch.HiddenInput /></Switch>
          </div>

          <!-- AFL cache -->
          <div class="flex items-center gap-4 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">AFL cache</div>
              <p class="mt-0.5 text-xs text-surface-400">
                <span class="font-mono tabular-nums text-surface-50">{cacheCount}</span> shows cached, refreshed {formatCacheTime(cacheRefreshedAt)}.
              </p>
            </div>
            <button class="btn btn-base preset-tonal shrink-0" onclick={handleRefreshCache} disabled={refreshingCache}>
              {#if refreshingCache}
                <LoaderCircle size={16} class="animate-spin" />
              {:else}
                <RefreshCw size={16} />
              {/if}
              {refreshingCache ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>
        </div>
      </section>

      <!-- Defaults group -->
      <section class="card preset-tonal-surface border-surface-800">
        <div class="card-body p-5 gap-0">
          <h2 class="text-sm font-mono uppercase tracking-wider text-surface-400 mb-3">Defaults</h2>
          <p class="text-xs text-surface-400 mb-3">These tracking rules are applied automatically when you add a new anime mapping. You can override them per anime.</p>

          <div class="flex items-center gap-4 border-b border-surface-800 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">Anime canon</div>
              <p class="mt-0.5 text-xs text-surface-400">Original anime episodes</p>
            </div>
            <Switch checked={trackAnimeCanonDefault} onCheckedChange={(d) => (trackAnimeCanonDefault = d.checked)}><Switch.Control><Switch.Thumb /></Switch.Control><Switch.HiddenInput /></Switch>
          </div>

          <div class="flex items-center gap-4 border-b border-surface-800 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">Mixed canon</div>
              <p class="mt-0.5 text-xs text-surface-400">Canon + filler blend</p>
            </div>
            <Switch checked={trackMixedDefault} onCheckedChange={(d) => (trackMixedDefault = d.checked)}><Switch.Control><Switch.Thumb /></Switch.Control><Switch.HiddenInput /></Switch>
          </div>

          <div class="flex items-center gap-4 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="text-[13px] font-medium text-surface-50">Filler</div>
              <p class="mt-0.5 text-xs text-surface-400">Pure filler episodes</p>
            </div>
            <Switch checked={trackFillerDefault} onCheckedChange={(d) => (trackFillerDefault = d.checked)}><Switch.Control><Switch.Thumb /></Switch.Control><Switch.HiddenInput /></Switch>
          </div>
        </div>
      </section>
    </div>
  {/if}
</div>
