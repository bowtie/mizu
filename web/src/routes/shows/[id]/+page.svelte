<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { Badge, Box, Button, Popover, Switch, Toggle, Tooltip } from '@foxui/core';
  import NumberInput from '../../../components/NumberInput.svelte';
  import { addNotification } from '../../../lib/stores.js';
  import { deleteShow, getAniListArtwork, getEpisodes, getSettings, getShows, getSonarrSeries, setEpisodeMonitor, updateShow } from '../../../lib/api.js';
  import { episodeTypeBadge, monitorStateBadge } from '../../../lib/fox.js';
  import { resolveSonarrImageUrl } from '../../../lib/sonarr-images.js';
  import { Bot, Check, Eye, LoaderCircle, Save, Trash2, Waves, X } from 'lucide-svelte';
  import { List, Settings2 } from 'lucide-svelte';

  const WATERMARK_AUTO = 'auto';
  const WATERMARK_MANUAL = 'manual';
  const TAB_OVERVIEW = 'overview';
  const TAB_EPISODES = 'episodes';
  let activeTab = TAB_OVERVIEW;

  let loading = true;
  let saving = false;
  let deleting = false;
  let showDeleteModal = false;
  let show = null;
  let series = null;
  let artwork = null;
  let sonarrBaseUrl = '';
  let episodes = [];
  let trackFiller = false;
  let trackMixed = true;
  let trackAnimeCanon = true;
  let watermarkMode = WATERMARK_AUTO;
  let watermarkAutoPressed = true;
  let markerValue = 0;
  let activeTypeFilters = [];
  let syncingEps = {};
  let pristineFiller = false;
  let pristineMixed = true;
  let pristineAnimeCanon = true;
  let pristineWatermarkAuto = true;
  let pristineMarkerValue = 0;

  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  const breakdownItems = [
    { label: 'Total', key: 'total', tone: 'text-base-50' },
    { label: 'Manga', key: 'canon', tone: 'text-blue-300' },
    { label: 'Anime', key: 'animeCanon', tone: 'text-violet-300' },
    { label: 'Mixed', key: 'mixed', tone: 'text-amber-300' },
    { label: 'Filler', key: 'filler', tone: 'text-rose-300' },
  ];

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

  $: showId = $page.params.id;
  $: pageTitle = show ? `Mizu - ${show.sonarr_title}` : 'Mizu - Anime';
  $: rulesDirty = !!show && (
    trackFiller !== pristineFiller ||
    trackMixed !== pristineMixed ||
    trackAnimeCanon !== pristineAnimeCanon ||
    watermarkAutoPressed !== pristineWatermarkAuto ||
    (watermarkAutoPressed === false && markerValue !== pristineMarkerValue)
  );
  $: filteredEpisodes = episodes.filter((ep) => (activeTypeFilters.length === 0 ? true : activeTypeFilters.includes(ep.type)));
  $: groupedBySeason = (() => {
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
  })();
  $: epStats = {
    total: episodes.length,
    canon: episodes.filter((ep) => ep.type === 'canon').length,
    mixed: episodes.filter((ep) => ep.type === 'mixed_canon').length,
    animeCanon: episodes.filter((ep) => ep.type === 'anime_canon').length,
    filler: episodes.filter((ep) => ep.type === 'filler').length,
    special: episodes.filter((ep) => ep.type === 'special').length,
  };
  $: markerUpperBound = episodes.length > 0
    ? episodes.reduce((highest, ep) => Math.max(highest, displayEpisodeNumber(ep)), 0)
    : undefined;
  $: activeBackdropUrl = artwork?.banner_image || artwork?.poster_image || backdropUrl(series);
  $: fullSubtitle = series?.overview || '';
  $: subtitle = briefOverview(fullSubtitle);
  $: watermarkMode = watermarkAutoPressed ? WATERMARK_AUTO : WATERMARK_MANUAL;
  $: watermarkTooltip = watermarkAutoPressed
    ? 'Using Sonarr progress as the start point.'
    : 'Using the episode input as the start point.';

  onMount(async () => {
    await loadShow();
  });

  async function loadShow() {
    loading = true;
    try {
      const [appSettings, allShows, allSeries, episodesResult] = await Promise.all([
        getSettings(),
        getShows(),
        getSonarrSeries(),
        getEpisodes(showId),
      ]);
      const artworkResult = appSettings?.enable_anilist_artwork ? await getAniListArtwork(showId).catch(() => null) : null;

      show = (allShows || []).find((item) => String(item.id) === String(showId));
      if (!show) {
        addNotification('Anime not found', 'error');
        goto('/shows');
        return;
      }

      sonarrBaseUrl = appSettings?.sonarr_url || '';
      series = (allSeries || []).find((item) => item.id === show.sonarr_series_id) || null;
      artwork = artworkResult?.banner_image || artworkResult?.poster_image ? artworkResult : null;
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
      addNotification('Anime rules updated', 'success');
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
      addNotification('Anime removed', 'success');
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
      addNotification(`${episodeLabel(ep)} ${nextState ? 'monitored' : 'unmonitored'}`, 'success');
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

  function filterButtonClasses(type, tone) {
    const active = isTypeFilterActive(type);
    const hasScopedFilter = activeTypeFilters.length > 0;
    const isDimmed = type !== 'all' && hasScopedFilter && !active;
    const toneClasses = {
      base: 'border-base-800/80 bg-base-950/60 text-base-100',
      blue: 'border-blue-500/20 bg-blue-950/20 text-blue-200',
      violet: 'border-violet-500/20 bg-violet-950/20 text-violet-200',
      amber: 'border-amber-500/20 bg-amber-950/20 text-amber-200',
      rose: 'border-rose-500/20 bg-rose-950/20 text-rose-200',
    };

    return [
      'rounded-2xl border p-4 text-left transition-colors',
      toneClasses[tone] || toneClasses.base,
      active ? 'ring-1 ring-accent-500/50' : '',
      isDimmed ? 'border-base-800/60 bg-base-950/40 text-base-500' : 'hover:border-base-700 hover:bg-base-900/70',
      type === 'all' && hasScopedFilter && !active ? 'border-accent-500/30 bg-accent-950/10 text-accent-200' : '',
    ].filter(Boolean).join(' ');
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

{#if loading}
  <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-2xl border border-base-800/80 bg-base-950/60 text-center">
    <LoaderCircle size={30} class="animate-spin text-accent-400" />
    <div class="space-y-1">
      <h2 class="text-base font-semibold text-base-100">Loading anime details</h2>
      <p class="text-sm text-base-400">Pulling mapping rules, episode data, and artwork.</p>
    </div>
  </Box>
{:else if !show}
  <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-2xl border border-base-800/80 bg-base-950/60 px-6 text-center">
    <span class="flex size-12 items-center justify-center rounded-xl border border-accent-500/20 bg-accent-950/30 text-accent-300">
      <Waves size={24} />
    </span>
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-base-50">Anime not found</h2>
      <p class="text-sm leading-6 text-base-400">The requested sync entry could not be found.</p>
    </div>
    <Button href="/shows" variant="primary">Back to anime</Button>
  </Box>
{:else}
  <div class="space-y-6">
    <!-- Backdrop hero (always visible) -->
    <Box class="overflow-hidden rounded-2xl border border-base-800/80 bg-base-950/60 p-0">
      {#if activeBackdropUrl}
        <div class="relative">
          <img src={activeBackdropUrl} alt={show.sonarr_title} class="h-48 w-full object-cover opacity-65 sm:h-56" />
          <div class="absolute inset-0 bg-gradient-to-t from-base-950 via-base-950/70 to-base-950/10"></div>
          <div class="absolute inset-x-0 bottom-0 p-5">
            <h1 class="text-2xl font-bold tracking-tight text-base-50 sm:text-3xl">{show.sonarr_title}</h1>
            {#if subtitle}
              <p class="mt-2 max-w-3xl text-sm leading-6 text-base-200" title={fullSubtitle}>{subtitle}</p>
            {/if}
          </div>
        </div>
      {:else}
        <div class="space-y-3 p-6">
          <h1 class="text-2xl font-bold tracking-tight text-base-50 sm:text-3xl">{show.sonarr_title}</h1>
          {#if subtitle}
            <p class="max-w-3xl text-sm leading-6 text-base-300">{subtitle}</p>
          {/if}
        </div>
      {/if}
    </Box>

    <!-- Tab bar -->
    <div class="flex items-center gap-1 rounded-xl border border-base-800/80 bg-base-950/60 p-1">
      <button
        type="button"
        class="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors {activeTab === TAB_OVERVIEW ? 'bg-base-800 text-base-50' : 'text-base-400 hover:text-base-200'}"
        onclick={() => (activeTab = TAB_OVERVIEW)}
      >
        <Settings2 size={16} />
        Overview
        {#if rulesDirty}
          <span class="size-1.5 rounded-full bg-accent-400"></span>
        {/if}
      </button>
      <button
        type="button"
        class="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors {activeTab === TAB_EPISODES ? 'bg-base-800 text-base-50' : 'text-base-400 hover:text-base-200'}"
        onclick={() => (activeTab = TAB_EPISODES)}
      >
        <List size={16} />
        Episodes
        <span class="text-xs text-base-500">{epStats.total}</span>
      </button>
    </div>

    <!-- Overview tab -->
    {#if activeTab === TAB_OVERVIEW}
      <section class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_21rem]">
        <div class="space-y-4">
          <Box class="space-y-4 rounded-2xl border border-base-800/80 bg-base-950/60">
            <div class="space-y-1">
              <h2 class="text-base font-semibold text-base-50">Tracking rules</h2>
              <p class="text-sm text-base-400">Choose which episode types should remain monitored for this anime.</p>
            </div>

            <div class="grid gap-3 md:grid-cols-3">
              <div class="rounded-xl border border-base-800/80 bg-base-950/70 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-base-100">Anime canon</div>
                    <div class="mt-1 text-sm text-base-400">Original anime-only canon episodes.</div>
                  </div>
                  <Switch bind:checked={trackAnimeCanon} />
                </div>
              </div>

              <div class="rounded-xl border border-base-800/80 bg-base-950/70 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-base-100">Mixed canon</div>
                    <div class="mt-1 text-sm text-base-400">Episodes that blend canon and filler material.</div>
                  </div>
                  <Switch bind:checked={trackMixed} />
                </div>
              </div>

              <div class="rounded-xl border border-base-800/80 bg-base-950/70 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-sm font-medium text-base-100">Filler</div>
                    <div class="mt-1 text-sm text-base-400">Pure filler episodes for this mapping.</div>
                  </div>
                  <Switch bind:checked={trackFiller} />
                </div>
              </div>
            </div>
          </Box>

          <!-- Episode stats summary for overview -->
          <Box class="rounded-2xl border border-base-800/80 bg-base-950/60 p-5">
            <div class="space-y-3">
              <h2 class="text-base font-semibold text-base-50">Episode breakdown</h2>
              <div class="grid grid-cols-2 gap-3 sm:grid-cols-5">
                {#each breakdownItems as item}
                  <div class="rounded-xl border border-base-800/80 bg-base-950/70 p-3 text-center">
                    <div class="text-2xl font-bold tabular-nums {item.tone}">{epStats[item.key]}</div>
                    <div class="mt-1 text-xs text-base-400">{item.label}</div>
                  </div>
                {/each}
              </div>
              <Button variant="ghost" size="sm" onclick={() => (activeTab = TAB_EPISODES)} class="-ml-2">
                View all episodes
                <List size={14} />
              </Button>
            </div>
          </Box>
        </div>

        <div class="space-y-4">
          <Box class="space-y-4 rounded-2xl border border-base-800/80 bg-base-950/60">
            {#if rulesDirty}
              <div class="rounded-lg border border-accent-500/20 bg-accent-950/20 px-3 py-2 text-sm text-accent-200">
                You have unsaved changes.
              </div>
            {/if}
            <div class="flex items-center gap-3">
              <label for="marker-value" class="sr-only">Start tracking after episode</label>
              <NumberInput
                id="marker-value"
                min="0"
                max={markerUpperBound}
                step="1"
                bind:value={markerValue}
                defaultValue={0}
                disabled={watermarkAutoPressed}
                className="flex-1"
              />
              <Tooltip
                withContext
                delayDuration={120}
                text={watermarkTooltip}
                contentProps={{ side: 'bottom', align: 'center', sideOffset: 10 }}
              >
                {#snippet child({ props })}
                  <Toggle
                    {...props}
                    bind:pressed={watermarkAutoPressed}
                    size="default"
                    class="cursor-pointer"
                    aria-label="Toggle automatic start from Sonarr"
                  >
                    <Bot size={16} />
                  </Toggle>
                {/snippet}
              </Tooltip>
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <Popover bind:open={showDeleteModal} side="top" align="center" class="rounded-[1.35rem] border-base-700/80 bg-base-900/98 px-4 py-3 text-base-100 shadow-xl">
                {#snippet child({ props })}
                  <Button {...props} variant="red" disabled={deleting}>
                    <Trash2 size={16} />
                    Remove
                  </Button>
                {/snippet}

                <div class="flex items-center gap-3">
                  <p class="pr-1 text-sm font-medium text-base-50">Remove {show?.sonarr_title || 'this anime'}?</p>
                  <div class="ml-auto flex items-center gap-2">
                    <Button
                      variant="secondary"
                      size="iconSm"
                      class="rounded-full"
                      disabled={deleting}
                      aria-label="Cancel removal"
                      onclick={() => (showDeleteModal = false)}
                    >
                      <X size={14} />
                    </Button>
                    <Button
                      variant="blue"
                      size="iconSm"
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
              </Popover>
              <Button variant="primary" onclick={handleSave} disabled={saving || !rulesDirty}>
                {#if saving}
                  <LoaderCircle size={16} class="animate-spin" />
                {:else}
                  <Save size={16} />
                {/if}
                {saving ? 'Saving...' : 'Save rules'}
              </Button>
            </div>
          </Box>
        </div>
      </section>
    {/if}

    <!-- Episodes tab -->
    {#if activeTab === TAB_EPISODES}
      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <button type="button" class={filterButtonClasses('all', 'base')} onclick={() => toggleTypeFilter('all')}>
          <div class="text-xs font-medium uppercase tracking-[0.12em] text-base-500">All episodes</div>
          <div class="mt-2 text-2xl font-semibold text-base-50">{epStats.total}</div>
        </button>
        <button type="button" class={filterButtonClasses('canon', 'blue')} onclick={() => toggleTypeFilter('canon')}>
          <div class="text-xs font-medium uppercase tracking-[0.12em] text-current/70">Manga</div>
          <div class="mt-2 text-2xl font-semibold">{epStats.canon}</div>
        </button>
        <button type="button" class={filterButtonClasses('anime_canon', 'violet')} onclick={() => toggleTypeFilter('anime_canon')}>
          <div class="text-xs font-medium uppercase tracking-[0.12em] text-current/70">Anime</div>
          <div class="mt-2 text-2xl font-semibold">{epStats.animeCanon}</div>
        </button>
        <button type="button" class={filterButtonClasses('mixed_canon', 'amber')} onclick={() => toggleTypeFilter('mixed_canon')}>
          <div class="text-xs font-medium uppercase tracking-[0.12em] text-current/70">Mixed</div>
          <div class="mt-2 text-2xl font-semibold">{epStats.mixed}</div>
        </button>
        <button type="button" class={filterButtonClasses('filler', 'rose')} onclick={() => toggleTypeFilter('filler')}>
          <div class="text-xs font-medium uppercase tracking-[0.12em] text-current/70">Filler</div>
          <div class="mt-2 text-2xl font-semibold">{epStats.filler}</div>
        </button>
      </section>

      {#if episodes.length === 0}
        <Box class="rounded-2xl border border-base-800/80 bg-base-950/60 p-8 text-center">
          <h2 class="text-lg font-semibold text-base-50">No episodes yet</h2>
          <p class="mt-2 text-sm leading-6 text-base-400">Mizu has not synced episode details for this anime yet.</p>
        </Box>
      {:else}
        <section class="space-y-4">
          {#each groupedBySeason as group, index}
            <details class="overflow-hidden rounded-2xl border border-base-800/80 bg-base-950/60" open={index === 0}>
              <summary class="cursor-pointer list-none border-b border-base-800/80 px-5 py-4 marker:hidden">
                <div class="flex flex-wrap items-center gap-3">
                  <h2 class="text-base font-semibold text-base-50">{seasonLabel(group.season)}</h2>
                  <Badge variant="secondary" size="sm">{group.episodes.length} episodes</Badge>
                </div>
              </summary>

              <div class="hidden overflow-x-auto md:block">
                <table class="min-w-full border-collapse text-left text-sm">
                  <thead class="border-b border-base-800/80 text-xs uppercase tracking-[0.12em] text-base-500">
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
                      <tr class="border-b border-base-900/80 last:border-b-0">
                        <td class="px-5 py-4 font-medium text-base-200">{displayEpisodeNumber(ep)}</td>
                        <td class="px-5 py-4 text-base-100">{ep.title || '-'}</td>
                        <td class="px-5 py-4">
                          <Badge variant={typeBadge.variant} size="sm">{typeBadge.label}</Badge>
                        </td>
                        <td class="px-5 py-4">
                          {#if episodeActionId(ep)}
                            <Badge variant={monitoredBadge.variant} size="sm">{monitoredBadge.label}</Badge>
                          {:else}
                            <span class="text-sm text-base-500">Unavailable</span>
                          {/if}
                        </td>
                        <td class="px-5 py-4 text-right">
                          {#if episodeActionId(ep)}
                            <Button
                              variant={episodeMonitored(ep) ? 'green' : 'secondary'}
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
                  <div class="rounded-xl border border-base-800/80 bg-base-950/70 p-4">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                      <div class="space-y-2">
                        <div class="text-sm font-medium text-base-200">{episodeLabel(ep)}</div>
                        <h3 class="text-sm font-semibold leading-5 text-base-50">{ep.title || '-'}</h3>
                      </div>
                      <Badge variant={typeBadge.variant} size="sm">{typeBadge.label}</Badge>
                    </div>

                    <div class="mt-4 flex items-center justify-between gap-3">
                      {#if episodeActionId(ep)}
                        <Badge variant={monitoredBadge.variant} size="sm">{monitoredBadge.label}</Badge>
                        <Button
                          variant={episodeMonitored(ep) ? 'green' : 'secondary'}
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
                        <span class="text-sm text-base-500">Episode monitoring unavailable.</span>
                      {/if}
                    </div>
                  </div>
                {/each}
              </div>
            </details>
          {/each}
        </section>
      {/if}
    {/if}
  </div>
{/if}
