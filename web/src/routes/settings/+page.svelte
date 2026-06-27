<script>
  import { onMount } from 'svelte';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Checkbox } from '$lib/components/ui/checkbox/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import Box from '$lib/components/Box.svelte';
  import { settings, addNotification } from '../../lib/stores.js';
  import { getSettings, getStoredSonarrApiKey, refreshAflCache, testSonarrConnection, updateSettings } from '../../lib/api.js';
  import { Eye, EyeOff, LoaderCircle, Plug, RefreshCw, Save } from 'lucide-svelte';

  let loading = true;
  let saving = false;
  let testing = false;
  let refreshingCache = false;
  let revealingKey = false;
  let testResult = null;

  let sonarrUrl = '';
  let sonarrApiKey = '';
  let savedApiKeyMasked = '';
  let hasSavedApiKey = false;
  let apiKeyVisible = false;
  let apiKeyDirty = false;
  let syncInterval = '6h';
  let enableTagging = false;
  let enableAniListArtwork = false;
  let fillerTagName = 'filler';
  let trackFillerDefault = false;
  let trackMixedDefault = true;
  let trackAnimeCanonDefault = true;
  let cacheRefreshedAt = null;
  let cacheCount = 0;

  // Pristine snapshots for dirty tracking
  let pristine = {};

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
        enableAniListArtwork = data.enable_anilist_artwork || false;
        fillerTagName = data.filler_tag_name || 'filler';
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
          enableTagging,
          enableAniListArtwork,
          fillerTagName,
          trackFillerDefault,
          trackMixedDefault,
          trackAnimeCanonDefault,
        };
      }
    } catch (err) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
    loading = false;
  }

  $: settingsDirty = !loading && (
    sonarrUrl !== pristine.sonarrUrl ||
    apiKeyDirty ||
    syncInterval !== pristine.syncInterval ||
    enableTagging !== pristine.enableTagging ||
    enableAniListArtwork !== pristine.enableAniListArtwork ||
    fillerTagName !== pristine.fillerTagName ||
    trackFillerDefault !== pristine.trackFillerDefault ||
    trackMixedDefault !== pristine.trackMixedDefault ||
    trackAnimeCanonDefault !== pristine.trackAnimeCanonDefault
  );

  $: urlValid = !sonarrUrl || /^https?:\/\/.+/.test(sonarrUrl);

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
        enable_anilist_artwork: enableAniListArtwork,
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
    if (!urlValid) {
      addNotification('URL must start with http:// or https://', 'warning');
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

<div class="space-y-6">
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
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-2xl border border-border/80 bg-popover/60 text-center">
      <LoaderCircle size={30} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">Loading settings</h2>
        <p class="text-sm text-muted-foreground">Pulling current Sonarr and sync configuration.</p>
      </div>
    </Box>
  {:else}
    <div class="mx-auto max-w-4xl space-y-5">
      {#if settingsDirty}
        <div class="flex items-center gap-2 rounded-xl border border-primary/20 bg-primary/10 px-4 py-2.5 text-sm text-primary">
          You have unsaved changes.
        </div>
      {/if}

      <!-- Connection section (critical) -->
      <Box class="space-y-5 rounded-2xl border border-border/80 bg-popover/60">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold text-foreground">Connection</h2>
            <Badge variant="secondary">Required</Badge>
            {#if testResult}
              <Badge variant={testResult.success ? 'success' : 'destructive'}>{testResult.message}</Badge>
            {/if}
          </div>
          <Button variant="secondary" onclick={handleTestConnection} disabled={testing || !sonarrUrl || !urlValid}>
            {#if testing}
              <LoaderCircle size={16} class="animate-spin" />
            {:else}
              <Plug size={16} />
            {/if}
            {testing ? 'Testing...' : 'Test connection'}
          </Button>
        </div>

        <div class="space-y-4">
          <div class="space-y-2">
            <Label for="sonarr-url">Sonarr URL</Label>
            <Input id="sonarr-url" type="text" placeholder="http://localhost:8989" bind:value={sonarrUrl} class="h-11 w-full" />
            {#if sonarrUrl && !urlValid}
              <p class="text-sm text-destructive">URL must start with http:// or https://</p>
            {/if}
          </div>

          <div class="space-y-2">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <Label for="sonarr-key">API key</Label>
              {#if hasSavedApiKey || apiKeyDirty}
                <Button type="button" variant="ghost" size="sm" onclick={toggleApiKeyVisibility} disabled={revealingKey}>
                  {#if apiKeyVisible}
                    <EyeOff size={16} />
                    Hide
                  {:else}
                    <Eye size={16} />
                    {revealingKey ? 'Loading...' : 'View'}
                  {/if}
                </Button>
              {/if}
            </div>

            <Input
              id="sonarr-key"
              type={apiKeyVisible ? 'text' : 'password'}
              bind:value={sonarrApiKey}
              placeholder={hasSavedApiKey ? 'Saved API key' : 'Your Sonarr API key'}
              oninput={handleApiKeyInput}
              class="h-11 w-full"
            />
          </div>
        </div>
      </Box>

      <!-- Sync section -->
      <Box class="space-y-5 rounded-2xl border border-border/80 bg-popover/60">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold text-foreground">Sync</h2>
            <Badge variant="secondary">{cacheCount} cached</Badge>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">Last refreshed: {formatCacheTime(cacheRefreshedAt)}</Badge>
            <Button variant="secondary" onclick={handleRefreshCache} disabled={refreshingCache}>
              {#if refreshingCache}
                <LoaderCircle size={16} class="animate-spin" />
              {:else}
                <RefreshCw size={16} />
              {/if}
              {refreshingCache ? 'Refreshing...' : 'Refresh'}
            </Button>
          </div>
        </div>

        <div class="space-y-4">
          <div class="space-y-2">
            <Label for="sync-interval">Sync interval</Label>
            <Select.Root type="single" bind:value={syncInterval}>
              <Select.Trigger id="sync-interval" class="w-full">
                {#if syncInterval}
                  {@const selected = syncIntervalOptions.find(i => i.value === syncInterval)}
                  {selected?.label || 'Choose an interval'}
                {:else}
                  <span class="text-muted-foreground">Choose an interval</span>
                {/if}
              </Select.Trigger>
              <Select.Content>
                {#each syncIntervalOptions as item}
                  <Select.Item value={item.value} label={item.label} />
                {/each}
              </Select.Content>
            </Select.Root>
            <p class="text-sm text-muted-foreground">How often Mizu checks for episode changes and updates Sonarr.</p>
          </div>

          <label class="flex items-start gap-3 rounded-xl border border-border/80 bg-popover/70 px-4 py-3">
            <Checkbox bind:checked={enableTagging} />
            <div class="space-y-1">
              <div class="text-sm font-medium text-foreground">Sonarr tagging</div>
              <div class="text-sm text-muted-foreground">Apply a filler tag to episodes inside Sonarr.</div>
            </div>
          </label>

          {#if enableTagging}
            <div class="space-y-2">
              <Label for="tag-name">Tag name</Label>
              <Input id="tag-name" type="text" placeholder="filler" bind:value={fillerTagName} class="h-11 w-full" />
            </div>
          {/if}

          <label class="flex items-start gap-3 rounded-xl border border-border/80 bg-popover/70 px-4 py-3">
            <Checkbox bind:checked={enableAniListArtwork} />
            <div class="space-y-1">
              <div class="text-sm font-medium text-foreground">AniList artwork</div>
              <div class="text-sm text-muted-foreground">Use AniList banners and posters instead of Sonarr images only.</div>
            </div>
          </label>
        </div>
      </Box>

      <!-- Defaults section -->
      <Box class="space-y-5 rounded-2xl border border-border/80 bg-popover/60">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-lg font-semibold text-foreground">Defaults</h2>
          <Badge variant="secondary">New anime</Badge>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">These tracking rules are applied automatically when you add a new anime mapping. You can override them per anime.</p>

        <div class="grid gap-3 md:grid-cols-3">
          <div class="flex items-center justify-between gap-3 rounded-xl border border-border/80 bg-popover/70 px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">Anime canon</div>
              <div class="mt-0.5 text-xs text-muted-foreground">Original anime episodes</div>
            </div>
            <Switch bind:checked={trackAnimeCanonDefault} />
          </div>

          <div class="flex items-center justify-between gap-3 rounded-xl border border-border/80 bg-popover/70 px-4 py-3">
            <div>
              <div class="text-sm font-medium text-foreground">Mixed canon</div>
              <div class="mt-0.5 text-xs text-muted-foreground">Canon + filler blend</div>
            </div>
            <Switch bind:checked={trackMixedDefault} />
          </div>

          <div class="flex items-center justify-between gap-3 rounded-xl border border-border/80 bg-popover/70 px-4 py-3">
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
