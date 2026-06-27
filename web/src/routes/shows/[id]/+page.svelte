<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Toggle } from '$lib/components/ui/toggle/index.js';
  import * as Popover from '$lib/components/ui/popover/index.js';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';
  import Box from '$lib/components/Box.svelte';
  import NumberInput from '../../../components/NumberInput.svelte';
  import { addNotification, shows as showsStore, settings as settingsStore } from '../../../lib/stores.js';
  import { deleteShow, getAniListArtwork, getEpisodes, getSettings, getShows, getSonarrSeries, setEpisodeMonitor, updateShow } from '../../../lib/api.js';
  import { get } from 'svelte/store';
  import { episodeTypeBadge, monitorStateBadge } from '../../../lib/fox.js';
  import { resolveSonarrImageUrl } from '../../../lib/sonarr-images.js';
  import { Bot, Check, Eye, LoaderCircle, Save, Trash2, Waves, X } from 'lucide-svelte';

  const WATERMARK_AUTO = 'auto';
  const WATERMARK_MANUAL = 'manual';

  let loading = $state(true);
  let saving = $state(false);
  let deleting = $state(false);
  let showDeleteModal = $state(false);
  let show = $state(null);
  let series = $state(null);
  let artwork = $state(null);
  let sonarrBaseUrl = $state('');
  let episodes = $state([]);
  let trackFiller = $state(false);
  let trackMixed = $state(true);
  let trackAnimeCanon = $state(true);
  let watermarkAutoPressed = $state(true);
  let markerValue = $state(0);
  let activeTypeFilters = $state([]);
  let syncingEps = $state({});
  let pristineFiller = false;
  let pristineMixed = true;
  let pristineAnimeCanon = true;
  let pristineWatermarkAuto = true;
  let pristineMarkerValue = 0;

  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  function briefOverview(copy) {
    const normalized = (copy || '').replace(/\s+/g, ' ').trim();
    if (!normalized) return '';
    if (normalized.length <= 170) return normalized;

    const sentenceBreak = normalized.match(/^(.{0,170}[.!?])(?:\s|$)/);
    if (sentenceBreak?.[1]) return sentenceBreak[1].trim();

    const truncated = normalized.slice(0, 167);
    const safeBoundary = truncated.lastIndexOf(' ');
    return `${(safeBoundary > 96 ? truncated.slice(0, safeBoundary) : truncated).trim()}...`;
  }

  let showId = $derived($page.params.id);
  let pageTitle = $derived(show ? `Mizu - ${show.sonarr_title}` : 'Mizu - Anime');
  let rulesDirty = $derived(!!show && (
    trackFiller !== pristineFiller ||
    trackMixed !== pristineMixed ||
    trackAnimeCanon !== pristineAnimeCanon ||
    watermarkAutoPressed !== pristineWatermarkAuto ||
    (watermarkAutoPressed === false && markerValue !== pristineMarkerValue)
  ));
  let filteredEpisodes = $derived(episodes.filter((ep) => (activeTypeFilters.length === 0 ? true : activeTypeFilters.includes(ep.type))));
  let groupedBySeason = $derived((() => {
    const groups = {};
    for (const ep of filteredEpisodes) {
      const season = episodeSeason(ep);
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }

    return Object.entries(groups)
      .map(([seasonKey, groupedEpisodes]) => ({
        season: Number.parseInt(seasonKey, 10),
        episodes: (groupedEpisodes || []).slice().sort((a, b) => displayEpisodeNumber(b) - displayEpisodeNumber(a)),
      }))
      .sort((a, b) => {
        if (a.season === -1) return 1;
        if (b.season === -1) return -1;
        return b.season - a.season;
      });
  })());
  let epStats = $derived({
    total: episodes.length,
    canon: episodes.filter((ep) => ep.type === 'canon').length,
    mixed: episodes.filter((ep) => ep.type === 'mixed_canon').length,
    animeCanon: episodes.filter((ep) => ep.type === 'anime_canon').length,
    filler: episodes.filter((ep) => ep.type === 'filler').length,
    special: episodes.filter((ep) => ep.type === 'special').length,
  });
  let markerUpperBound = $derived(episodes.length > 0
    ? episodes.reduce((highest, ep) => Math.max(highest, displayEpisodeNumber(ep)), 0)
    : undefined);
  let activeBackdropUrl = $derived(artwork?.banner_image || artwork?.poster_image || backdropUrl(series));
  let fullSubtitle = $derived(series?.overview || '');
  let subtitle = $derived(briefOverview(fullSubtitle));
  let watermarkMode = $derived(watermarkAutoPressed ? WATERMARK_AUTO : WATERMARK_MANUAL);
  let watermarkTooltip = $derived(watermarkAutoPressed
    ? 'Using Sonarr progress as the start point.'
    : 'Using the episode input as the start point.');

  onMount(async () => {
    await loadShow();
  });

  async function loadShow() {
    loading = true;
    try {
      // Use cached stores from Shows page when available
      let cachedShows = get(showsStore);
      let cachedSettings = get(settingsStore);
      const hasCachedShows = cachedShows?.length > 0;
      const hasCachedSettings = cachedSettings?.sonarr_url !== undefined;

      // Only fetch shows/settings if not in store, fetch episodes + series in parallel
      const [showsResult, settingsResult, episodesResult] = await Promise.all([
        hasCachedShows ? Promise.resolve(cachedShows) : getShows(),
        hasCachedSettings ? Promise.resolve(cachedSettings) : getSettings(),
        getEpisodes(showId),
      ]);

      const appSettings = settingsResult || cachedSettings || {};
      if (!hasCachedSettings && settingsResult) settingsStore.set(settingsResult);
      if (!hasCachedShows && showsResult) showsStore.set(showsResult);

      show = (showsResult || cachedShows || []).find((item) => String(item.id) === String(showId));
      if (!show) {
        addNotification('Anime not found', 'error');
        goto('/shows');
        return;
      }

      sonarrBaseUrl = appSettings?.sonarr_url || '';

      // Use cached AniList artwork from mapping if available, otherwise fetch from API
      if (appSettings?.enable_anilist_artwork) {
        if (show.anilist_banner_url || show.anilist_poster_url) {
          artwork = {
            banner_image: show.anilist_banner_url || '',
            poster_image: show.anilist_poster_url || '',
          };
        } else {
          const artworkResult = await getAniListArtwork(showId).catch(() => null);
          artwork = artworkResult?.banner_image || artworkResult?.poster_image ? artworkResult : null;
        }
      } else {
        artwork = null;
      }

      // Always fetch series for overview/description; also used as backdrop fallback
      getSonarrSeries()
        .then((allSeries) => {
          series = (allSeries || []).find((item) => item.id === show.sonarr_series_id) || null;
        })
        .catch(() => {});

      episodes = episodesResult || [];
      trackFiller = actionToTracked(show.filler_action, false);
      trackMixed = actionToTracked(show.mixed_action, true);
      trackAnimeCanon = actionToTracked(show.anime_canon_action, true);
      watermarkAutoPressed = (show.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO;
      markerValue = clampMarkerValue(show.sync_watermark || 0, episodesResult || []);
      pristineFiller = trackFiller;
      pristineMixed = trackMixed;
      pristineAnimeCanon = trackAnimeCanon;
      pristineWatermarkAuto = watermarkAutoPressed;
      pristineMarkerValue = markerValue;
    } catch (err) {
      addNotification('Failed to load anime: ' + err.message, 'error');
    }
    loading = false;
  }

  async function handleSave() {
    saving = true;
    try {
      const payload = {
        filler_action: trackedToAction(trackFiller),
        mixed_action: trackedToAction(trackMixed),
        anime_canon_action: trackedToAction(trackAnimeCanon),
        watermark_mode: watermarkMode,
      };
      if (watermarkMode === WATERMARK_MANUAL) {
        payload.sync_watermark = clampMarkerValue(markerValue, episodes);
      }
      await updateShow(showId, payload);
      addNotification(`Saved tracking rules for ${show?.sonarr_title || 'anime'}`, 'success');
      await loadShow();
    } catch (err) {
      addNotification('Failed to save anime: ' + err.message, 'error');
    }
    saving = false;
  }

  async function handleDelete() {
    deleting = true;
    try {
      await deleteShow(showId);
      addNotification(`Removed ${show?.sonarr_title || 'anime'} from Mizu`, 'success');
      showDeleteModal = false;
      goto('/shows');
    } catch (err) {
      addNotification('Failed to remove anime: ' + err.message, 'error');
    }
    deleting = false;
  }

  async function toggleEpisodeMonitor(ep) {
    const sonarrEp = resolveSonarrEpisode(ep);
    if (!sonarrEp?.id) {
      addNotification('No Sonarr episode found for ' + episodeLabel(ep), 'error');
      return;
    }
    syncingEps[sonarrEp.id] = true;
    syncingEps = syncingEps;
    try {
      const nextState = !sonarrEp.monitored;
      await setEpisodeMonitor(showId, sonarrEp.id, nextState);
      if (ep?.sonarr_episode_id) {
        ep.monitored = nextState;
        episodes = episodes;
      }
      addNotification(`${episodeLabel(ep)} ${nextState ? 'now monitored' : 'now unmonitored'}`, 'success');
    } catch (err) {
      addNotification('Failed to update episode: ' + err.message, 'error');
    }
    delete syncingEps[sonarrEp.id];
    syncingEps = syncingEps;
  }

  function backdropUrl(series) {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((image) => image.coverType === 'fanart') || series.images.find((image) => image.coverType === 'banner') || series.images.find((image) => image.coverType === 'poster') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, sonarrBaseUrl);
  }

  function toggleTypeFilter(type) {
    if (type === 'all') {
      activeTypeFilters = [];
      return;
    }

    if (activeTypeFilters.includes(type)) {
      activeTypeFilters = activeTypeFilters.filter((value) => value !== type);
      return;
    }

    activeTypeFilters = [...activeTypeFilters, type];
  }

  function isTypeFilterActive(type) {
    if (type === 'all') return activeTypeFilters.length === 0;
    return activeTypeFilters.includes(type);
  }

  let filterButtons = $derived([
    { type: 'all', label: 'All episodes', count: epStats.total, active: isTypeFilterActive('all'), activeClass: 'border-primary bg-primary text-primary-foreground' },
    { type: 'canon', label: 'Manga', count: epStats.canon, active: isTypeFilterActive('canon'), activeClass: 'border-primary bg-primary text-primary-foreground' },
    { type: 'anime_canon', label: 'Anime', count: epStats.animeCanon, active: isTypeFilterActive('anime_canon'), activeClass: 'border-info bg-info text-info-foreground' },
    { type: 'mixed_canon', label: 'Mixed', count: epStats.mixed, active: isTypeFilterActive('mixed_canon'), activeClass: 'border-warning bg-warning text-warning-foreground' },
    { type: 'filler', label: 'Filler', count: epStats.filler, active: isTypeFilterActive('filler'), activeClass: 'border-destructive bg-destructive text-destructive-foreground' },
  ]);

  function isSpecialSeasonNumber(num) {
    return num === -1 || num === 0;
  }

  function seasonLabel(num) {
    return isSpecialSeasonNumber(num) ? 'Specials' : `Season ${num}`;
  }

  function resolveSonarrEpisode(ep) {
    if (!ep?.sonarr_episode_id) return null;
    return {
      id: ep.sonarr_episode_id,
      monitored: !!ep.monitored,
    };
  }

  function episodeSeason(ep) {
    const rawSeason = Number.isInteger(ep?.season_number) ? ep.season_number : -1;
    if (isSpecialSeasonNumber(rawSeason)) return -1;
    return Number.isInteger(rawSeason) ? rawSeason : -1;
  }

  function episodeMonitored(ep) {
    return !!resolveSonarrEpisode(ep)?.monitored;
  }

  function episodeActionId(ep) {
    return resolveSonarrEpisode(ep)?.id || null;
  }

  function displayEpisodeNumber(ep) {
    if (isSpecialSeasonNumber(ep?.season_number)) return ep?.sonarr_episode_number || 0;
    return ep?.absolute_episode_number ?? ep?.episode_num ?? ep?.sonarr_episode_number ?? 0;
  }

  function episodeLabel(ep) {
    return isSpecialSeasonNumber(ep?.season_number)
      ? `Special ${displayEpisodeNumber(ep)}`
      : `Episode #${displayEpisodeNumber(ep)}`;
  }

  function clampMarkerValue(nextValue, sourceEpisodes = episodes) {
    const normalized = Math.max(0, Number(nextValue) || 0);
    if (!sourceEpisodes?.length) return normalized;
    const upperBound = sourceEpisodes.reduce((highest, ep) => Math.max(highest, displayEpisodeNumber(ep)), 0);
    return Math.min(normalized, upperBound);
  }
</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

{#snippet headerActions()}
  <div class="flex shrink-0 flex-col items-end gap-2">
    <div class="flex items-center gap-2">
      <Tooltip.Provider delayDuration={120}>
        <Tooltip.Root>
          <Tooltip.Trigger>
            <Toggle
              bind:pressed={watermarkAutoPressed}
              size="default"
              class="h-11 min-w-11 cursor-pointer"
              aria-label="Toggle automatic start from Sonarr"
            >
              <Bot size={16} />
            </Toggle>
          </Tooltip.Trigger>
          <Tooltip.Content side="bottom" align="center" sideOffset={10}>
            {watermarkTooltip}
          </Tooltip.Content>
        </Tooltip.Root>
      </Tooltip.Provider>
      <label for="marker-value" class="sr-only">Start tracking after episode</label>
      <NumberInput
        id="marker-value"
        min="0"
        max={markerUpperBound}
        step="1"
        bind:value={markerValue}
        defaultValue={0}
        disabled={watermarkAutoPressed}
        className="w-24"
      />
    </div>
    <div class="flex items-center gap-2">
    <Popover.Root bind:open={showDeleteModal}>
      <Popover.Trigger>
        <Button variant="destructive" disabled={deleting}>
          <Trash2 size={16} />
          Remove
        </Button>
      </Popover.Trigger>
      <Popover.Content side="bottom" align="end" class="rounded-[1.35rem] border-border/80 bg-card/98 px-4 py-3 text-foreground shadow-xl">
        <div class="flex items-center gap-3">
          <p class="pr-1 text-sm font-medium text-foreground">Remove {show?.sonarr_title || 'this anime'}?</p>
          <div class="ml-auto flex items-center gap-2">
            <Button
              variant="secondary"
              size="icon-sm"
              class="rounded-full"
              disabled={deleting}
              aria-label="Cancel removal"
              onclick={() => (showDeleteModal = false)}
            >
              <X size={14} />
            </Button>
            <Button
              variant="default"
              size="icon-sm"
              class="rounded-full"
              disabled={deleting}
              aria-label="Confirm removal"
              onclick={handleDelete}
            >
              {#if deleting}
                <LoaderCircle size={16} class="animate-spin" />
              {:else}
                <Check size={14} />
              {/if}
            </Button>
          </div>
        </div>
      </Popover.Content>
    </Popover.Root>
    <Button variant="default" onclick={handleSave} disabled={saving || !rulesDirty}>
      {#if saving}
        <LoaderCircle size={16} class="animate-spin" />
      {:else}
        <Save size={16} />
      {/if}
      {saving ? 'Saving...' : 'Save'}
    </Button>
    </div>
  </div>
{/snippet}

{#if loading}
  <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-2xl border border-border/80 bg-popover/60 text-center">
    <LoaderCircle size={30} class="animate-spin text-primary" />
    <div class="space-y-1">
      <h2 class="text-base font-semibold text-foreground">Loading anime details</h2>
      <p class="text-sm text-muted-foreground">Pulling mapping rules, episode data, and artwork.</p>
    </div>
  </Box>
{:else if !show}
  <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-2xl border border-border/80 bg-popover/60 px-6 text-center">
    <span class="flex size-12 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
      <Waves size={24} />
    </span>
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-foreground">Anime not found</h2>
      <p class="text-sm leading-6 text-muted-foreground">The requested sync entry could not be found.</p>
    </div>
    <Button href="/shows" variant="default">Back to anime</Button>
  </Box>
{:else}
  <div class="space-y-6">
    <!-- Backdrop hero with Save + Remove actions -->
    <Box class="overflow-hidden rounded-2xl border border-border/80 bg-popover/60 p-0">
      {#if activeBackdropUrl}
        <div class="relative">
          <img src={activeBackdropUrl} alt={show.sonarr_title} class="h-48 w-full object-cover opacity-65 sm:h-56" />
          <div class="absolute inset-0 bg-gradient-to-t from-popover via-popover/70 to-popover/10"></div>
          <div class="absolute inset-x-0 bottom-0 flex items-end justify-between gap-4 p-5">
            <div class="min-w-0">
              <h1 class="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">{show.sonarr_title}</h1>
              {#if subtitle}
                <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground" title={fullSubtitle}>{subtitle}</p>
              {/if}
            </div>
            {@render headerActions()}
          </div>
        </div>
      {:else}
        <div class="flex items-end justify-between gap-4 space-y-0 p-6">
          <div class="space-y-3">
            <h1 class="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">{show.sonarr_title}</h1>
            {#if subtitle}
              <p class="max-w-3xl text-sm leading-6 text-muted-foreground">{subtitle}</p>
            {/if}
          </div>
          {@render headerActions()}
        </div>
      {/if}
    </Box>

    <!-- Overview -->
    <section class="space-y-4">
      <Box class="space-y-4 rounded-2xl border border-border/80 bg-popover/60 p-5">
        <div class="flex items-center justify-between gap-3">
          <div class="space-y-1">
            <h2 class="text-base font-semibold text-foreground">Tracking rules</h2>
            <p class="text-sm text-muted-foreground">Choose which episode types should remain monitored for this anime.</p>
          </div>
          {#if rulesDirty}
            <span class="rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-xs font-medium text-primary">Unsaved changes</span>
          {/if}
        </div>

        <div class="grid gap-3 md:grid-cols-3">
              <div class="rounded-xl border border-border/80 bg-popover/70 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-foreground">Anime canon</div>
                    <div class="mt-1 text-sm text-muted-foreground">Original anime-only canon episodes.</div>
                  </div>
                  <Switch bind:checked={trackAnimeCanon} />
                </div>
              </div>

              <div class="rounded-xl border border-border/80 bg-popover/70 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-foreground">Mixed canon</div>
                    <div class="mt-1 text-sm text-muted-foreground">Episodes that blend canon and filler material.</div>
                  </div>
                  <Switch bind:checked={trackMixed} />
                </div>
              </div>

              <div class="rounded-xl border border-border/80 bg-popover/70 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-foreground">Filler</div>
                    <div class="mt-1 text-sm text-muted-foreground">Pure filler episodes for this mapping.</div>
                  </div>
                  <Switch bind:checked={trackFiller} />
                </div>
              </div>
            </div>
          </Box>
    </section>

    <!-- Episodes -->
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      {#each filterButtons as btn (btn.type)}
        {@const dimmed = btn.type !== 'all' && activeTypeFilters.length > 0 && !btn.active}
        <button
          type="button"
          class="rounded-2xl border p-4 text-left transition-all {btn.active
            ? btn.activeClass
            : 'border-border bg-secondary text-foreground hover:border-primary/50'} {dimmed ? 'opacity-40' : ''}"
          onclick={() => toggleTypeFilter(btn.type)}
        >
          <div class="text-xs font-medium uppercase tracking-[0.12em] opacity-70">{btn.label}</div>
          <div class="mt-2 text-2xl font-semibold">{btn.count}</div>
        </button>
      {/each}
    </section>

    {#if episodes.length === 0}
      <Box class="rounded-2xl border border-border/80 bg-popover/60 p-8 text-center">
        <h2 class="text-lg font-semibold text-foreground">No episodes yet</h2>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">Mizu has not synced episode details for this anime yet.</p>
      </Box>
    {:else}
      <section class="space-y-4">
        {#each groupedBySeason as group, index}
          <details class="overflow-hidden rounded-2xl border border-border/80 bg-popover/60" open={index === 0}>
            <summary class="cursor-pointer list-none px-5 py-4 marker:hidden open:border-b open:border-border/80">
              <div class="flex flex-wrap items-center gap-3">
                <h2 class="text-base font-semibold text-foreground">{seasonLabel(group.season)}</h2>
                <Badge variant="secondary">{group.episodes.length} episodes</Badge>
              </div>
            </summary>

            <div class="hidden overflow-x-auto md:block">
              <table class="min-w-full border-collapse text-left text-sm">
                <thead class="border-b border-border/80 text-xs uppercase tracking-[0.12em] text-muted-foreground">
                  <tr>
                    <th class="px-5 py-3 font-medium">#</th>
                    <th class="px-5 py-3 font-medium">Title</th>
                    <th class="px-5 py-3 font-medium">Type</th>
                    <th class="px-5 py-3 font-medium">Monitored</th>
                    <th class="px-5 py-3 font-medium text-right">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {#each group.episodes as ep}
                    {@const typeBadge = episodeTypeBadge(ep.type)}
                    {@const monitoredBadge = monitorStateBadge(episodeMonitored(ep), 'Yes', 'No')}
                    <tr class="border-b border-border/80 last:border-b-0">
                      <td class="px-5 py-4 font-medium text-muted-foreground">{displayEpisodeNumber(ep)}</td>
                      <td class="px-5 py-4 text-foreground">{ep.title || '-'}</td>
                      <td class="px-5 py-4">
                        <Badge variant={typeBadge.variant}>{typeBadge.label}</Badge>
                      </td>
                      <td class="px-5 py-4">
                        {#if episodeActionId(ep)}
                          <Badge variant={monitoredBadge.variant}>{monitoredBadge.label}</Badge>
                        {:else}
                          <span class="text-sm text-muted-foreground">Unavailable</span>
                        {/if}
                      </td>
                      <td class="px-5 py-4 text-right">
                        {#if episodeActionId(ep)}
                          <Button
                            variant={episodeMonitored(ep) ? 'default' : 'secondary'}
                            size="sm"
                            disabled={syncingEps[episodeActionId(ep)]}
                            onclick={() => toggleEpisodeMonitor(ep)}
                          >
                            {#if syncingEps[episodeActionId(ep)]}
                              <LoaderCircle size={14} class="animate-spin" />
                            {:else}
                              <Eye size={14} />
                            {/if}
                            {episodeMonitored(ep) ? 'Monitored' : 'Monitor'}
                          </Button>
                        {/if}
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <div class="space-y-3 p-4 md:hidden">
              {#each group.episodes as ep}
                {@const typeBadge = episodeTypeBadge(ep.type)}
                {@const monitoredBadge = monitorStateBadge(episodeMonitored(ep))}
                <div class="rounded-xl border border-border/80 bg-popover/70 p-4">
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div class="space-y-2">
                      <div class="text-sm font-medium text-muted-foreground">{episodeLabel(ep)}</div>
                      <h3 class="text-sm font-semibold leading-5 text-foreground">{ep.title || '-'}</h3>
                    </div>
                    <Badge variant={typeBadge.variant}>{typeBadge.label}</Badge>
                  </div>

                  <div class="mt-4 flex items-center justify-between gap-3">
                    {#if episodeActionId(ep)}
                      <Badge variant={monitoredBadge.variant}>{monitoredBadge.label}</Badge>
                      <Button
                        variant={episodeMonitored(ep) ? 'default' : 'secondary'}
                        size="sm"
                        disabled={syncingEps[episodeActionId(ep)]}
                        onclick={() => toggleEpisodeMonitor(ep)}
                      >
                        {#if syncingEps[episodeActionId(ep)]}
                          <LoaderCircle size={14} class="animate-spin" />
                        {:else}
                          <Eye size={14} />
                        {/if}
                        {episodeMonitored(ep) ? 'Monitored' : 'Monitor'}
                      </Button>
                    {:else}
                      <span class="text-sm text-muted-foreground">Episode monitoring unavailable.</span>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          </details>
        {/each}
      </section>
    {/if}
  </div>
{/if}
