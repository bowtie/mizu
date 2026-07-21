<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
  import { AnimatePresence, motion } from '@humanspeak/svelte-motion';
  import { Badge } from '$lib/components/ui/badge/index.ts';
  import { Button } from '$lib/components/ui/button/index.ts';
  import { Input } from '$lib/components/ui/input/index.ts';
  import { Label } from '$lib/components/ui/label/index.ts';
  import { Switch } from '$lib/components/ui/switch/index.ts';
  import Box from '$lib/components/Box.svelte';
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

<div class="space-y-8">
  <!-- Header -->
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">Settings</h1>
      <p class="mt-1 text-sm text-muted-foreground">Configure Sonarr connection, sync behavior, and defaults.</p>
    </div>
    <Button variant="default" onclick={handleSave} disabled={saving || !settingsDirty}>
      {#if saving}
        <LoaderCircle size={16} class="animate-spin" />
      {:else}
        <Save size={16} />
      {/if}
      {saving ? 'Saving...' : 'Save settings'}
    </Button>
  </section>

  {#if loading}
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-xl glass-card text-center">
      <LoaderCircle size={30} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">Loading settings</h2>
        <p class="text-sm text-muted-foreground">Pulling current Sonarr and sync configuration.</p>
      </div>
    </Box>
  {:else}
    <div class="space-y-5">
      {#if settingsDirty}
        <div class="flex items-center gap-2 rounded-xl border border-primary/20 bg-primary/10 px-4 py-2.5 text-sm text-primary">
          You have unsaved changes.
        </div>
      {/if}

      <!-- Connection section (critical) -->
      <Box class="space-y-5 rounded-xl glass-card p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold text-foreground">Connection</h2>
            <Badge variant="secondary">Required</Badge>
          </div>
          <Button
            variant={testPhase === 'success' ? 'default' : testPhase === 'error' ? 'destructive' : 'secondary'}
            onclick={handleTestConnection}
            disabled={testing || testPhase === 'success' || !sonarrUrl || !urlValid}
            aria-busy={testing}
            data-state={testPhase}
            class="relative {testPhase === 'success' ? 'bg-success text-success-foreground hover:bg-success/80 disabled:opacity-100' : ''}"
          >
            <!-- Invisible widest label reserves stable width -->
            <span class="invisible flex items-center gap-2" aria-hidden="true">
              <span class="size-4"></span>
              Retry connection
            </span>
            <!-- Content: AnimatePresence mode="wait" = exit completes, then enter starts -->
            <span class="absolute inset-0 flex items-center justify-center gap-2">
              <AnimatePresence mode="wait" initial={false}>
                {#if testPhase === 'connecting'}
                  <motion.span
                    key="connecting"
                    class="flex items-center gap-2"
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -6 }}
                    transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                  >
                    <LoaderCircle size={16} class="animate-spin" />
                    Testing
                  </motion.span>
                {:else if testPhase === 'success'}
                  <motion.span
                    key="success"
                    class="flex items-center gap-2"
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -6 }}
                    transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                  >
                    <Check size={16} />
                    Connected
                  </motion.span>
                {:else if testPhase === 'error'}
                  <motion.span
                    key="error"
                    class="flex items-center gap-2"
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -6 }}
                    transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                  >
                    <AlertTriangle size={16} />
                    Retry connection
                  </motion.span>
                {:else}
                  <motion.span
                    key="idle"
                    class="flex items-center gap-2"
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -6 }}
                    transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                  >
                    <Plug size={16} />
                    Test connection
                  </motion.span>
                {/if}
              </AnimatePresence>
            </span>
          </Button>
        </div>

        {#if testPhase === 'error' && testErrorMessage}
          <p class="text-sm text-destructive" transition:slide={slideIn(200)}>
            {testErrorMessage}
          </p>
        {/if}

        <div class="space-y-4">
          <div class="space-y-2">
            <Label for="sonarr-url">Sonarr URL</Label>
            <Input id="sonarr-url" type="text" placeholder="http://localhost:8989" value={sonarrUrl} oninput={handleUrlInput} class="w-full" />
            {#if sonarrUrl && !urlValid}
              <p class="text-sm text-destructive">URL must start with http:// or https://</p>
            {/if}
          </div>

          <div class="space-y-2">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <Label for="sonarr-key">API key</Label>
              {#if hasSavedApiKey || apiKeyDirty}
                <Button type="button" variant="ghost" size="sm" onclick={toggleApiKeyVisibility} disabled={revealingKey} class="relative">
                  <span class="invisible flex items-center gap-1.5" aria-hidden="true">
                    <Eye size={16} />
                    View
                  </span>
                  <span class="absolute inset-0 flex items-center justify-center">
                    <AnimatePresence mode="wait" initial={false}>
                      {#if apiKeyVisible}
                        <motion.span key="hide" class="flex items-center gap-1.5" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.15 }}>
                          <EyeOff size={16} />
                          Hide
                        </motion.span>
                      {:else if revealingKey}
                        <motion.span key="loading" class="flex items-center gap-1.5" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.15 }}>
                          <LoaderCircle size={16} class="animate-spin" />
                        </motion.span>
                      {:else}
                        <motion.span key="view" class="flex items-center gap-1.5" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.15 }}>
                          <Eye size={16} />
                          View
                        </motion.span>
                      {/if}
                    </AnimatePresence>
                  </span>
                </Button>
              {/if}
            </div>

            <Input
              id="sonarr-key"
              type={apiKeyVisible ? 'text' : 'password'}
              bind:value={sonarrApiKey}
              placeholder={hasSavedApiKey ? 'Saved API key' : 'Your Sonarr API key'}
              oninput={handleApiKeyInput}
              class="w-full"
            />
          </div>
        </div>
      </Box>

      <!-- Sync section -->
      <Box class="space-y-5 rounded-xl glass-card p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold text-foreground">Sync</h2>
            <Badge variant="secondary">{cacheCount} cached</Badge>
            {#if cacheRefreshedAt}
              <span class="text-xs text-muted-foreground">{formatCacheTime(cacheRefreshedAt)}</span>
            {/if}
          </div>
          <Button variant="secondary" onclick={handleRefreshCache} disabled={refreshingCache}>
            {#if refreshingCache}
              <LoaderCircle size={16} class="animate-spin" />
            {:else}
              <RefreshCw size={16} />
            {/if}
            {refreshingCache ? 'Refreshing...' : 'Refresh'}
          </Button>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div class="flex items-center justify-between gap-3 rounded-xl glass-card px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">Sync interval</div>
              <div class="text-sm text-muted-foreground">How often Mizu checks for episode changes and updates Sonarr.</div>
            </div>
            <div class="flex shrink-0 items-center rounded-lg border border-primary/20 bg-secondary/40 text-foreground transition focus-within:border-primary/50 focus-within:ring-2 focus-within:ring-primary/25">
              <button
                type="button"
                class="flex h-8 w-8 items-center justify-center rounded-l-lg text-muted-foreground transition hover:bg-primary/10 hover:text-foreground disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
                onclick={() => stepSyncInterval(-1)}
                disabled={syncIntervalIndex <= 0}
                aria-label="Decrease sync interval"
              >
                <Minus size={16} />
              </button>
              <div class="h-8 min-w-[5rem] flex items-center justify-center px-2 text-center text-sm font-semibold text-foreground">
                {syncIntervalOptions[syncIntervalIndex].label}
              </div>
              <button
                type="button"
                class="flex h-8 w-8 items-center justify-center rounded-r-lg text-muted-foreground transition hover:bg-primary/10 hover:text-foreground disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
                onclick={() => stepSyncInterval(1)}
                disabled={syncIntervalIndex >= syncIntervalOptions.length - 1}
                aria-label="Increase sync interval"
              >
                <Plus size={16} />
              </button>
            </div>
          </div>

          <div class="flex items-center justify-between gap-3 rounded-xl glass-card px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">AniList artwork</div>
              <div class="text-sm text-muted-foreground">Use AniList banners and posters instead of Sonarr images only.</div>
            </div>
            <Switch bind:checked={enableAniListArtwork} />
          </div>
        </div>
      </Box>

      <!-- Defaults section -->
      <Box class="space-y-5 rounded-xl glass-card p-5">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-lg font-semibold text-foreground">Defaults</h2>
          <Badge variant="secondary">New anime</Badge>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">These tracking rules are applied automatically when you add a new anime mapping. You can override them per anime.</p>

        <div class="grid gap-3 md:grid-cols-3">
          <div class="flex items-center justify-between gap-3 rounded-xl glass-card px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">Anime canon</div>
              <div class="mt-0.5 text-xs text-muted-foreground">Original anime episodes</div>
            </div>
            <Switch bind:checked={trackAnimeCanonDefault} />
          </div>

          <div class="flex items-center justify-between gap-3 rounded-xl glass-card px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">Mixed canon</div>
              <div class="mt-0.5 text-xs text-muted-foreground">Canon + filler blend</div>
            </div>
            <Switch bind:checked={trackMixedDefault} />
          </div>

          <div class="flex items-center justify-between gap-3 rounded-xl glass-card px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">Filler</div>
              <div class="mt-0.5 text-xs text-muted-foreground">Pure filler episodes</div>
            </div>
            <Switch bind:checked={trackFillerDefault} />
          </div>
        </div>
      </Box>
    </div>
  {/if}
</div>
