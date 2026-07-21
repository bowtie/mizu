<script lang="ts">
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { Button } from '$lib/components/ui/button/index.ts';
  import { Switch } from '$lib/components/ui/switch/index.ts';
  import { Toggle } from '$lib/components/ui/toggle/index.ts';
  import * as Popover from '$lib/components/ui/popover/index.ts';
  import * as Tooltip from '$lib/components/ui/tooltip/index.ts';
  import NumberInput from '../../../components/NumberInput.svelte';
  import { addNotification, shows as showsStore, settings as settingsStore } from '../../../lib/stores.ts';
  import { deleteShow, getAniListArtwork, getEpisodes, getSettings, getShows, getSonarrSeries, setEpisodeMonitor, updateShow } from '../../../lib/api.ts';
  import type { Artwork, Episode, Settings, Show, ShowUpdate, SonarrImage, SonarrSeries } from '$lib/types.ts';
  import { get } from 'svelte/store';
  import { episodeTypeBadge, monitorStateBadge } from '../../../lib/fox.ts';
  import { resolveSonarrImageUrl } from '../../../lib/sonarr-images.ts';
  import { Bot, Check, LoaderCircle, Save, Trash2, X } from 'lucide-svelte';

  const WATERMARK_AUTO = 'auto';
  const WATERMARK_MANUAL = 'manual';

  let loading: boolean = $state(true);
  let saving: boolean = $state(false);
  let savedFlash: boolean = $state(false);
  let deleting: boolean = $state(false);
  let showDeleteModal: boolean = $state(false);
  let show = $state<Show | null>(null);
  let series = $state<SonarrSeries | null>(null);
  let artwork = $state<Artwork | null>(null);
  let sonarrBaseUrl: string = $state('');
  let episodes = $state<Episode[]>([]);
  let openSeasons: Set<number> = $state(new Set([0]));
  let trackFiller: boolean = $state(false);
  let trackMixed: boolean = $state(true);
  let trackAnimeCanon: boolean = $state(true);
  let watermarkAutoPressed = $state(true) as boolean;
  let markerValue: number = $state(0);
  let activeTypeFilters: string[] = $state([]);
  let syncingEps: Record<string, boolean> = $state({});
  let pristineFiller: boolean = false;
  let pristineMixed: boolean = true;
  let pristineAnimeCanon: boolean = true;
  let pristineWatermarkAuto: boolean = true;
  let pristineMarkerValue: number = 0;

  const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean =>
    (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

  let showId: string = $derived($page.params.id || '');
  let pageTitle: string = $derived(show ? `Mizu - ${show.sonarr_title}` : 'Mizu - Anime');
  let rulesDirty: boolean = $derived(!!show && (
    trackFiller !== pristineFiller ||
    trackMixed !== pristineMixed ||
    trackAnimeCanon !== pristineAnimeCanon ||
    watermarkAutoPressed !== pristineWatermarkAuto ||
    (((watermarkAutoPressed as boolean) === false) && markerValue !== pristineMarkerValue)
  ));
  let filteredEpisodes: Episode[] = $derived(episodes.filter((ep: Episode) => (activeTypeFilters.length === 0 ? true : (ep.type !== undefined && activeTypeFilters.includes(ep.type)))));
  let groupedBySeason = $derived((() => {
    const groups: Record<string, Episode[]> = {};
    for (const ep of filteredEpisodes) {
      const season = String(episodeSeason(ep));
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }

    return Object.entries(groups)
      .map(([seasonKey, groupedEpisodes]) => ({
        season: Number.parseInt(seasonKey, 10),
        episodes: (groupedEpisodes || []).slice().sort((a: Episode, b: Episode) => displayEpisodeNumber(b) - displayEpisodeNumber(a)),
      }))
      .sort((a: { season: number; episodes: Episode[] }, b: { season: number; episodes: Episode[] }) => {
        if (a.season === -1) return 1;
        if (b.season === -1) return -1;
        return b.season - a.season;
      });
  })());
  let epStats = $derived({
    total: episodes.length,
    canon: episodes.filter((ep: Episode) => ep.type === 'canon').length,
    mixed: episodes.filter((ep: Episode) => ep.type === 'mixed_canon').length,
    animeCanon: episodes.filter((ep: Episode) => ep.type === 'anime_canon').length,
    filler: episodes.filter((ep: Episode) => ep.type === 'filler').length,
    special: episodes.filter((ep: Episode) => ep.type === 'special').length,
  });
  let markerUpperBound: number | undefined = $derived(episodes.length > 0
    ? episodes.reduce((highest: number, ep: Episode) => Math.max(highest, displayEpisodeNumber(ep)), 0)
    : undefined);
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
      let cachedShows: Show[] = get(showsStore);
      let cachedSettings: Settings = get(settingsStore);
      const hasCachedShows = cachedShows?.length > 0;
      const hasCachedSettings = cachedSettings?.sonarr_url !== undefined;

      // Only fetch shows/settings if not in store, fetch episodes + series in parallel
      const [showsResult, settingsResult, episodesResult] = await Promise.all([
        hasCachedShows ? Promise.resolve(cachedShows) : getShows(),
        hasCachedSettings ? Promise.resolve(cachedSettings) : getSettings(),
        getEpisodes(showId),
      ]) as [Show[], Settings, Episode[]];

      const appSettings = settingsResult || cachedSettings;
      if (!hasCachedSettings && settingsResult) settingsStore.set(settingsResult);
      if (!hasCachedShows && showsResult) showsStore.set(showsResult);

      const foundShow = (showsResult || cachedShows || []).find((item: Show) => String(item.id) === String(showId));
      if (!foundShow) {
        addNotification('Anime not found', 'error');
        goto('/shows');
        return;
      }
      show = foundShow;

      sonarrBaseUrl = appSettings?.sonarr_url || '';

      // Use cached AniList artwork from mapping if available, otherwise fetch from API
      if (appSettings?.enable_anilist_artwork) {
        if (foundShow.anilist_banner_url || foundShow.anilist_poster_url) {
          artwork = {
            banner_image: foundShow.anilist_banner_url || '',
            poster_image: foundShow.anilist_poster_url || '',
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
        .then((allSeries: SonarrSeries[]) => {
          series = (allSeries || []).find((item: SonarrSeries) => item.id === foundShow.sonarr_series_id) || null;
        })
        .catch(() => {});

      episodes = episodesResult || [];
      trackFiller = actionToTracked(foundShow.filler_action, false);
      trackMixed = actionToTracked(foundShow.mixed_action, true);
      trackAnimeCanon = actionToTracked(foundShow.anime_canon_action, true);
      watermarkAutoPressed = (foundShow.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO;
      markerValue = clampMarkerValue(foundShow.sync_watermark || 0, episodesResult || []);
      pristineFiller = trackFiller;
      pristineMixed = trackMixed;
      pristineAnimeCanon = trackAnimeCanon;
      pristineWatermarkAuto = watermarkAutoPressed;
      pristineMarkerValue = markerValue;
    } catch (err: any) {
      addNotification('Failed to load anime: ' + err.message, 'error');
    }
    loading = false;
  }

  async function handleSave() {
    saving = true;
    try {
      const payload: ShowUpdate = {
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
      savedFlash = true;
      setTimeout(() => (savedFlash = false), 1500);
      await loadShow();
    } catch (err: any) {
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
    } catch (err: any) {
      addNotification('Failed to remove anime: ' + err.message, 'error');
    }
    deleting = false;
  }

  async function toggleEpisodeMonitor(ep: Episode) {
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
      if (ep.sonarr_episode_id) {
        ep.monitored = nextState;
        episodes = episodes;
      }
      addNotification(`${episodeLabel(ep)} ${nextState ? 'now monitored' : 'now unmonitored'}`, 'success');
    } catch (err: any) {
      addNotification('Failed to update episode: ' + err.message, 'error');
    }
    delete syncingEps[sonarrEp.id];
    syncingEps = syncingEps;
  }

  function backdropUrl(series: SonarrSeries | null | undefined) {
    if (!series?.images?.length) return '';
    const preferred: SonarrImage | undefined = series.images.find((image: SonarrImage) => image.coverType === 'fanart') || series.images.find((image: SonarrImage) => image.coverType === 'banner') || series.images.find((image: SonarrImage) => image.coverType === 'poster') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, sonarrBaseUrl);
  }

  function toggleTypeFilter(type: string) {
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

  function isTypeFilterActive(type: string) {
    if (type === 'all') return activeTypeFilters.length === 0;
    return activeTypeFilters.includes(type);
  }

  let filterButtons = $derived([
    { type: 'all', label: 'All', count: epStats.total, active: isTypeFilterActive('all'), color: '#dee2e6' },
    { type: 'canon', label: 'Manga', count: epStats.canon, active: isTypeFilterActive('canon'), color: '#74c0fc' },
    { type: 'anime_canon', label: 'Anime', count: epStats.animeCanon, active: isTypeFilterActive('anime_canon'), color: '#66d9e8' },
    { type: 'mixed_canon', label: 'Mixed', count: epStats.mixed, active: isTypeFilterActive('mixed_canon'), color: '#ffa94d' },
    { type: 'filler', label: 'Filler', count: epStats.filler, active: isTypeFilterActive('filler'), color: '#ffa8a8' },
  ]);

  function isSpecialSeasonNumber(num: number | undefined) {
    return num === -1 || num === 0;
  }

  function seasonLabel(num: number | undefined) {
    return isSpecialSeasonNumber(num) ? 'Specials' : `Season ${num}`;
  }

  function resolveSonarrEpisode(ep: Episode) {
    if (!ep.sonarr_episode_id) return null;
    return {
      id: ep.sonarr_episode_id,
      monitored: !!ep.monitored,
    };
  }

  function episodeSeason(ep: Episode) {
    const rawSeason = Number.isInteger(ep.season_number) ? ep.season_number : -1;
    if (isSpecialSeasonNumber(rawSeason)) return -1;
    return Number.isInteger(rawSeason) ? rawSeason : -1;
  }

  function episodeMonitored(ep: Episode) {
    return !!resolveSonarrEpisode(ep)?.monitored;
  }

  function episodeActionId(ep: Episode) {
    return resolveSonarrEpisode(ep)?.id ?? 0;
  }

  function displayEpisodeNumber(ep: Episode) {
    if (isSpecialSeasonNumber(ep.season_number)) return ep.sonarr_episode_number || 0;
    return ep.absolute_episode_number ?? ep.episode_num ?? ep.sonarr_episode_number ?? 0;
  }

  function episodeLabel(ep: Episode) {
    return isSpecialSeasonNumber(ep.season_number)
      ? `Special ${displayEpisodeNumber(ep)}`
      : `Episode #${displayEpisodeNumber(ep)}`;
  }

  function clampMarkerValue(nextValue: number, sourceEpisodes: Episode[] = episodes) {
    const normalized = Math.max(0, Number(nextValue) || 0);
    if (!sourceEpisodes?.length) return normalized;
    const upperBound = sourceEpisodes.reduce((highest: number, ep: Episode) => Math.max(highest, displayEpisodeNumber(ep)), 0);
    return Math.min(normalized, upperBound);
  }

  function tagClassForType(type: string | undefined | null) {
    switch (type) {
      case 'canon':
        return 'tag-canon';
      case 'anime_canon':
        return 'tag-anime';
      case 'mixed_canon':
      case 'mixed':
        return 'tag-mixed';
      case 'filler':
        return 'tag-filler';
      default:
        return 'text-muted-foreground';
    }
  }
</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

{#snippet headerActions()}
  <div class="flex shrink-0 flex-wrap items-center justify-end gap-2">
    <div class="flex items-center gap-1.5">
      <Tooltip.Provider delayDuration={120}>
        <Tooltip.Root>
          <Tooltip.Trigger>
            <Toggle
              bind:pressed={watermarkAutoPressed}
              size="default"
              variant="outline"
              class="h-8 w-8 cursor-pointer rounded-lg px-0"
              aria-label="Toggle automatic start from Sonarr"
            >
              <Bot size={15} />
            </Toggle>
          </Tooltip.Trigger>
          <Tooltip.Content side="top" align="center" sideOffset={10} class="tooltip-animated">
            {watermarkTooltip}
          </Tooltip.Content>
        </Tooltip.Root>
      </Tooltip.Provider>
      <label for="marker-value-header" class="sr-only">Start tracking after episode</label>
      <NumberInput
        id="marker-value-header"
        min={0}
        max={markerUpperBound}
        step={1}
        bind:value={markerValue}
        defaultValue={0}
        disabled={watermarkAutoPressed}
        className="w-24"
      />
    </div>
    <span class="hidden h-6 w-px shrink-0 bg-rule sm:block" aria-hidden="true"></span>
    <div class="flex items-center gap-1.5">
    <Popover.Root bind:open={showDeleteModal}>
      <Popover.Trigger>
        <Button variant="outline" size="sm" disabled={deleting} class="text-destructive hover:text-destructive">
          <Trash2 size={14} />
          Remove
        </Button>
      </Popover.Trigger>
      <Popover.Content side="bottom" align="end" class="rounded-lg px-4 py-3 text-foreground">
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
    <Button variant="default" size="sm" onclick={handleSave} disabled={saving || !rulesDirty} class={savedFlash ? 'animate-in zoom-in-125 duration-300' : ''}>
      {#if saving}
        <LoaderCircle size={14} class="animate-spin" />
      {:else if savedFlash}
        <Check size={14} />
      {:else}
        <Save size={14} />
      {/if}
      {saving ? 'Saving...' : savedFlash ? 'Saved' : 'Save'}
    </Button>
    </div>
  </div>
{/snippet}

{#if loading}
  <div class="glass-surface flex min-h-[22rem] flex-col items-center justify-center gap-3 rounded-lg text-center">
    <LoaderCircle size={22} class="animate-spin text-primary" />
    <p class="font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">Loading anime details</p>
  </div>
{:else if !show}
  <div class="glass-surface flex min-h-[22rem] flex-col items-center justify-center gap-3 rounded-lg px-6 text-center">
    <h2 class="text-base font-semibold text-foreground">Anime not found</h2>
    <p class="text-sm text-muted-foreground">The requested sync entry could not be found.</p>
    <Button href="/shows" variant="default" size="sm">Back to anime</Button>
  </div>
{:else}
  <div class="space-y-6">
    <!-- Hero banner head -->
    <header class="relative overflow-hidden rounded-lg border border-rule">
      {#if backdropUrl(series)}
        <div class="absolute inset-0 z-0" aria-hidden="true">
          <img
            src={backdropUrl(series)}
            alt=""
            loading="lazy"
            decoding="async"
            class="h-full w-full object-cover opacity-[0.35]"
          />
          <div class="absolute inset-0 bg-gradient-to-t from-background via-background/80 to-background/30"></div>
        </div>
      {/if}
      <div class="relative z-10 flex flex-wrap items-end justify-between gap-4 px-5 py-6 sm:px-7 sm:py-8">
        <div class="min-w-0 space-y-1.5">
          <nav class="font-mono text-[11px] text-[#868e96]">
            <a href="/shows" class="transition-colors hover:text-muted-foreground">library</a>
            <span> / {show.afl_slug || show.sonarr_title}</span>
          </nav>
          <h1 class="text-2xl font-bold tracking-tight text-foreground">{show.sonarr_title}</h1>
          <p class="text-sm text-muted-foreground">{episodes.length} episodes &middot; from ep {show.sync_watermark}</p>
        </div>
        {@render headerActions()}
      </div>
    </header>

    <!-- Tracking rules -->
    <section class="space-y-4">
      <div class="flex items-center gap-3">
        <div class="rule-label flex-1"><span>Tracking rules</span></div>
        {#if rulesDirty}
          <span class="rounded-[5px] border border-[#ffa94d]/30 bg-[#ffa94d]/10 px-2 py-[3px] font-mono text-[10px] uppercase tracking-[0.1em] text-[#ffa94d]">Unsaved changes</span>
        {/if}
      </div>

      <div class="grid gap-3 md:grid-cols-3">
        <div class="glass-surface flex items-center justify-between gap-3 rounded-lg px-4 py-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-foreground">Anime canon</div>
            <div class="mt-0.5 text-xs text-muted-foreground">Original anime-only canon episodes.</div>
          </div>
          <Switch bind:checked={trackAnimeCanon} />
        </div>

        <div class="glass-surface flex items-center justify-between gap-3 rounded-lg px-4 py-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-foreground">Mixed canon</div>
            <div class="mt-0.5 text-xs text-muted-foreground">Episodes that blend canon and filler material.</div>
          </div>
          <Switch bind:checked={trackMixed} />
        </div>

        <div class="glass-surface flex items-center justify-between gap-3 rounded-lg px-4 py-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-foreground">Filler</div>
            <div class="mt-0.5 text-xs text-muted-foreground">Pure filler episodes for this mapping.</div>
          </div>
          <Switch bind:checked={trackFiller} />
        </div>
      </div>
    </section>

    <!-- Type filter stats -->
    <section class="grid grid-cols-3 gap-px overflow-hidden rounded-lg border border-rule bg-rule sm:grid-cols-5">
      {#each filterButtons as btn (btn.type)}
        {@const dimmed = btn.type !== 'all' && activeTypeFilters.length > 0 && !btn.active}
        <button
          type="button"
          class={`flex cursor-pointer items-center justify-between gap-2 bg-card px-4 py-2.5 text-left transition-colors duration-150 ${btn.active ? '' : 'text-muted-foreground hover:text-foreground'} ${dimmed ? 'opacity-40' : ''}`}
          style:color={btn.active ? btn.color : undefined}
          style:border-bottom={btn.active ? `2px solid ${btn.color}` : '2px solid transparent'}
          onclick={() => toggleTypeFilter(btn.type)}
        >
          <span class="font-mono text-[10px] uppercase tracking-[0.12em]">{btn.label}</span>
          <span class="font-mono text-sm font-medium tabular-nums">{btn.count}</span>
        </button>
      {/each}
    </section>

    {#if episodes.length === 0}
      <div class="glass-surface flex min-h-56 flex-col items-center justify-center gap-2 rounded-lg p-8 text-center">
        <h2 class="text-base font-semibold text-foreground">No episodes yet</h2>
        <p class="text-sm text-muted-foreground">Mizu has not synced episode details for this anime yet.</p>
      </div>
    {:else}
      <section class="space-y-5">
        {#each groupedBySeason as group, index}
          {@const isOpen = openSeasons.has(index)}
          <div>
            <button
              type="button"
              class="rule-label w-full cursor-pointer select-none"
              onclick={() => {
                const next = new Set(openSeasons);
                if (next.has(index)) next.delete(index);
                else next.add(index);
                openSeasons = next;
              }}
              aria-expanded={isOpen}
            >
              <span>{seasonLabel(group.season)} &middot; {group.episodes.length} eps</span>
            </button>

            {#if isOpen}
              <div transition:slide={{ duration: 200 }}>
                {#each group.episodes as ep (ep.sonarr_episode_id || ep.episode_num)}
                  {@const typeBadge = episodeTypeBadge(ep.type)}
                  {@const monitoredBadge = monitorStateBadge(episodeMonitored(ep), 'Yes', 'No')}
                  <div class="flex items-center gap-3 border-b border-[#10141b] py-2.5 transition-colors last:border-b-0 hover:bg-white/[0.02]">
                    <span class="w-8 shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">{displayEpisodeNumber(ep)}</span>
                    <span class="min-w-0 flex-1 truncate text-sm text-foreground">{ep.title || '-'}</span>
                    <span class="tag shrink-0 {tagClassForType(ep.type)}">{typeBadge.label}</span>
                    {#if episodeActionId(ep)}
                      <button
                        type="button"
                        class="inline-flex shrink-0 cursor-pointer items-center gap-1.5 rounded-full border border-rule px-3 py-1 font-mono text-[11px] transition-colors {episodeMonitored(ep) ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'}"
                        disabled={syncingEps[episodeActionId(ep)]}
                        onclick={() => toggleEpisodeMonitor(ep)}
                        title={monitoredBadge.label}
                      >
                        {#if syncingEps[episodeActionId(ep)]}
                          <LoaderCircle size={12} class="animate-spin" />
                        {:else}
                          <span class="size-1.5 rounded-full {episodeMonitored(ep) ? 'bg-[#12b886]' : 'bg-transparent ring-1 ring-[#868e96] ring-inset'}"></span>
                        {/if}
                        {episodeMonitored(ep) ? 'watch' : 'skip'}
                      </button>
                    {:else}
                      <span class="shrink-0 text-xs text-muted-foreground">Unavailable</span>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/each}
      </section>
    {/if}
  </div>
{/if}
