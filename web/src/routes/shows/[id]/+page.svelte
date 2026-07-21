<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import NumberInput from '../../../components/NumberInput.svelte';
  import { addNotification, shows as showsStore, settings as settingsStore } from '../../../lib/stores.ts';
  import { deleteShow, getAniListArtwork, getEpisodes, getSettings, getShows, getSonarrSeries, setEpisodeMonitor, updateShow } from '../../../lib/api.ts';
  import type { Artwork, Episode, Settings, Show, ShowUpdate, SonarrImage, SonarrSeries } from '$lib/types.ts';
  import { get } from 'svelte/store';
  import { episodeTypeBadge, monitorStateBadge } from '../../../lib/fox.ts';
  import { resolveSonarrImageUrl } from '../../../lib/sonarr-images.ts';
  import { Switch } from '@skeletonlabs/skeleton-svelte';
  import { Bot, Check, LoaderCircle, Save, Trash2 } from 'lucide-svelte';

  const WATERMARK_AUTO = 'auto';
  const WATERMARK_MANUAL = 'manual';

  let loading: boolean = $state(true);
  let saving: boolean = $state(false);
  let savedFlash: boolean = $state(false);
  let deleting: boolean = $state(false);
  let deleteDialog: HTMLDialogElement | null = $state(null);
  let show = $state<Show | null>(null);
  let series = $state<SonarrSeries | null>(null);
  let artwork = $state<Artwork | null>(null);
  let sonarrBaseUrl: string = $state('');
  let episodes = $state<Episode[]>([]);
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

  function openDeleteDialog() {
    deleteDialog?.showModal();
  }

  function closeDeleteDialog() {
    deleteDialog?.close();
  }

  async function handleDelete() {
    deleting = true;
    try {
      await deleteShow(showId);
      addNotification(`Removed ${show?.sonarr_title || 'anime'} from Mizu`, 'success');
      closeDeleteDialog();
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
    { type: 'all', label: 'All', count: epStats.total, active: isTypeFilterActive('all'), color: 'text-surface-200' },
    { type: 'canon', label: 'Manga', count: epStats.canon, active: isTypeFilterActive('canon'), color: 'text-primary-500' },
    { type: 'anime_canon', label: 'Anime', count: epStats.animeCanon, active: isTypeFilterActive('anime_canon'), color: 'text-tertiary-500' },
    { type: 'mixed_canon', label: 'Mixed', count: epStats.mixed, active: isTypeFilterActive('mixed_canon'), color: 'text-warning-500' },
    { type: 'filler', label: 'Filler', count: epStats.filler, active: isTypeFilterActive('filler'), color: 'text-error-500' },
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

  function badgePresetForType(type: string | undefined | null) {
    switch (type) {
      case 'canon':
        return 'preset-filled-primary-500';
      case 'anime_canon':
        return 'preset-filled-tertiary-500';
      case 'mixed_canon':
      case 'mixed':
        return 'preset-filled-warning-500';
      case 'filler':
        return 'preset-filled-error-500';
      default:
        return 'preset-tonal-surface';
    }
  }
</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

{#snippet headerActions()}
  <div class="flex shrink-0 flex-wrap items-center justify-end gap-2">
    <div class="flex items-center gap-1.5">
      <button
        type="button"
        class="btn-icon btn-icon-sm {watermarkAutoPressed ? 'preset-filled-primary-500' : 'preset-outlined'} cursor-pointer"
        onclick={() => (watermarkAutoPressed = !watermarkAutoPressed)}
        aria-label="Toggle automatic start from Sonarr"
        aria-pressed={watermarkAutoPressed}
        title={watermarkTooltip}
      >
        <Bot size={15} />
      </button>
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
    <span class="vr hidden sm:block" aria-hidden="true"></span>
    <div class="flex items-center gap-1.5">
      <button
        type="button"
        class="btn btn-sm preset-outlined-error-500 text-error-500"
        disabled={deleting}
        onclick={openDeleteDialog}
      >
        <Trash2 size={14} />
        Remove
      </button>
      <button
        type="button"
        class="btn btn-sm preset-filled-primary-500 {savedFlash ? 'animate-in zoom-in-125 duration-300' : ''}"
        onclick={handleSave}
        disabled={saving || !rulesDirty}
      >
        {#if saving}
          <LoaderCircle size={14} class="animate-spin" />
        {:else if savedFlash}
          <Check size={14} />
        {:else}
          <Save size={14} />
        {/if}
        {saving ? 'Saving...' : savedFlash ? 'Saved' : 'Save'}
      </button>
    </div>
  </div>
{/snippet}

<!-- Delete confirmation dialog -->
<dialog bind:this={deleteDialog} class="dialog animate-dialog preset-filled-surface-100-900" aria-labelledby="delete-dialog-title">
  <header>
    <h2 id="delete-dialog-title" class="h3">Remove {show?.sonarr_title || 'this anime'}?</h2>
  </header>
  <article>
    <p class="text-sm text-surface-400">This removes the mapping from Mizu. The series stays in Sonarr.</p>
  </article>
  <footer class="flex justify-end gap-2">
    <button type="button" class="btn btn-sm preset-tonal" disabled={deleting} onclick={closeDeleteDialog}>Cancel</button>
    <button type="button" class="btn btn-sm preset-filled-error-500" disabled={deleting} onclick={handleDelete}>
      {#if deleting}
        <LoaderCircle size={14} class="animate-spin" />
      {:else}
        <Trash2 size={14} />
      {/if}
      Remove
    </button>
  </footer>
</dialog>

{#if loading}
  <div class="card preset-tonal-surface flex min-h-[22rem] flex-col items-center justify-center gap-3 text-center">
    <LoaderCircle size={22} class="animate-spin text-primary-500" />
    <p class="font-mono text-[11px] uppercase tracking-[0.12em] text-surface-400">Loading anime details</p>
  </div>
{:else if !show}
  <div class="card preset-tonal-surface flex min-h-[22rem] flex-col items-center justify-center gap-3 px-6 text-center">
    <h2 class="text-base font-semibold text-surface-50">Anime not found</h2>
    <p class="text-sm text-surface-400">The requested sync entry could not be found.</p>
    <a href="/shows" class="btn btn-sm preset-filled-primary-500">Back to anime</a>
  </div>
{:else}
  <div class="space-y-6">
    <!-- Hero banner head -->
    <header class="card preset-filled-surface-100-900 relative overflow-hidden">
      {#if backdropUrl(series)}
        <div class="absolute inset-0 z-0" aria-hidden="true">
          <img
            src={backdropUrl(series)}
            alt=""
            loading="lazy"
            decoding="async"
            class="h-full w-full object-cover opacity-[0.35]"
          />
          <div class="absolute inset-0 bg-gradient-to-t from-surface-950 via-surface-950/80 to-surface-950/30"></div>
        </div>
      {/if}
      <div class="relative z-10 flex flex-wrap items-end justify-between gap-4 px-5 py-6 sm:px-7 sm:py-8">
        <div class="min-w-0 space-y-1.5">
          <nav class="font-mono text-[11px] text-surface-500">
            <a href="/shows" class="transition-colors hover:text-surface-300">library</a>
            <span> / {show.afl_slug || show.sonarr_title}</span>
          </nav>
          <h1 class="text-2xl font-bold tracking-tight text-surface-50">{show.sonarr_title}</h1>
          <p class="text-sm text-surface-400">{episodes.length} episodes &middot; from ep {show.sync_watermark}</p>
        </div>
        {@render headerActions()}
      </div>
    </header>

    <!-- Tracking rules -->
    <section class="space-y-4">
      <div class="flex items-center gap-3">
        <h2 class="h6 text-surface-200">Tracking rules</h2>
        <hr class="hr flex-1" />
        {#if rulesDirty}
          <span class="badge badge-sm font-mono text-[10px] uppercase tracking-[0.1em] preset-filled-warning-500">Unsaved changes</span>
        {/if}
      </div>

      <div class="grid gap-3 md:grid-cols-3">
        <div class="card preset-filled-surface-100-900 flex items-center justify-between gap-3 px-4 py-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-surface-50">Anime canon</div>
            <div class="mt-0.5 text-xs text-surface-400">Original anime-only canon episodes.</div>
          </div>
          <Switch checked={trackAnimeCanon} onCheckedChange={(d) => (trackAnimeCanon = d.checked)}>
            <Switch.Control>
              <Switch.Thumb />
            </Switch.Control>
            <Switch.HiddenInput />
          </Switch>
        </div>

        <div class="card preset-filled-surface-100-900 flex items-center justify-between gap-3 px-4 py-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-surface-50">Mixed canon</div>
            <div class="mt-0.5 text-xs text-surface-400">Episodes that blend canon and filler material.</div>
          </div>
          <Switch checked={trackMixed} onCheckedChange={(d) => (trackMixed = d.checked)}>
            <Switch.Control>
              <Switch.Thumb />
            </Switch.Control>
            <Switch.HiddenInput />
          </Switch>
        </div>

        <div class="card preset-filled-surface-100-900 flex items-center justify-between gap-3 px-4 py-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-surface-50">Filler</div>
            <div class="mt-0.5 text-xs text-surface-400">Pure filler episodes for this mapping.</div>
          </div>
          <Switch checked={trackFiller} onCheckedChange={(d) => (trackFiller = d.checked)}>
            <Switch.Control>
              <Switch.Thumb />
            </Switch.Control>
            <Switch.HiddenInput />
          </Switch>
        </div>
      </div>
    </section>

    <!-- Type filter stats -->
    <section class="card grid grid-cols-3 gap-px overflow-hidden sm:grid-cols-5" style="background: var(--color-surface-800);">
      {#each filterButtons as btn (btn.type)}
        {@const dimmed = btn.type !== 'all' && activeTypeFilters.length > 0 && !btn.active}
        <button
          type="button"
          class={`flex cursor-pointer items-center justify-between gap-2 bg-surface-900 px-4 py-2.5 text-left transition-colors duration-150 ${btn.active ? '' : 'text-surface-400 hover:text-surface-200'} ${dimmed ? 'opacity-40' : ''}`}
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
      <div class="card preset-tonal-surface flex min-h-56 flex-col items-center justify-center gap-2 p-8 text-center">
        <h2 class="text-base font-semibold text-surface-50">No episodes yet</h2>
        <p class="text-sm text-surface-400">Mizu has not synced episode details for this anime yet.</p>
      </div>
    {:else}
      <section class="disclosure-group space-y-2">
        {#each groupedBySeason as group, index}
          <details class="disclosure" open={index === 0}>
            <summary>{seasonLabel(group.season)} &middot; {group.episodes.length} eps</summary>
            <div class="disclosure-content">
              <div class="table-wrap">
                <table class="table">
                  <tbody class="[&>tr]:hover:preset-tonal-surface">
                    {#each group.episodes as ep (ep.sonarr_episode_id || ep.episode_num)}
                      {@const typeBadge = episodeTypeBadge(ep.type)}
                      {@const monitoredBadge = monitorStateBadge(episodeMonitored(ep), 'Yes', 'No')}
                      <tr>
                        <td class="w-10 font-mono text-[11px] tabular-nums text-surface-400">{displayEpisodeNumber(ep)}</td>
                        <td class="min-w-0 truncate text-sm text-surface-50">{ep.title || '-'}</td>
                        <td class="w-28 text-right">
                          <span class="badge badge-sm font-mono text-[10px] uppercase tracking-[0.08em] {badgePresetForType(ep.type)}">{typeBadge.label}</span>
                        </td>
                        <td class="w-28 text-right">
                          {#if episodeActionId(ep)}
                            <button
                              type="button"
                              class="btn btn-xs {episodeMonitored(ep) ? 'preset-tonal-success' : 'preset-tonal'} cursor-pointer"
                              disabled={syncingEps[episodeActionId(ep)]}
                              onclick={() => toggleEpisodeMonitor(ep)}
                              title={monitoredBadge.label}
                            >
                              {#if syncingEps[episodeActionId(ep)]}
                                <LoaderCircle size={12} class="animate-spin" />
                              {:else}
                                <span class="badge-dot {episodeMonitored(ep) ? 'preset-filled-success-500' : 'preset-outlined-surface-500'}"></span>
                              {/if}
                              {episodeMonitored(ep) ? 'watch' : 'skip'}
                            </button>
                          {:else}
                            <span class="text-xs text-surface-400">Unavailable</span>
                          {/if}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          </details>
        {/each}
      </section>
    {/if}
  </div>
{/if}
