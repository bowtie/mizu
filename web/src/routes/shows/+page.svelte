<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { ActionReturn } from 'svelte/action';
  import { fade, scale, fly } from 'svelte/transition';
  import { autoAnimateAction as autoAnimate } from '../../lib/autoAnimate.ts';
  import { goto } from '$app/navigation';
  import type { AFLMatch, MappingCreate, SelectOption, Settings, Show, SonarrAddOptions, SonarrAddPayload, SonarrImage, SonarrSeries } from '$lib/types.ts';
  import { Button } from '$lib/components/ui/button/index.ts';
  import { Checkbox } from '$lib/components/ui/checkbox/index.ts';
  import { Input } from '$lib/components/ui/input/index.ts';
  import { Label } from '$lib/components/ui/label/index.ts';
  import { ScrollArea } from '$lib/components/ui/scroll-area/index.ts';
  import * as Select from '$lib/components/ui/select/index.ts';
  import { shows, settings, addNotification } from '../../lib/stores.ts';
  import {
    addSonarrSeries,
    getShows,
    getSonarrAddOptions,
    getSonarrSeries,
    lookupSonarrSeries,
    markManualMatchPending,
    suggestAFL,
    searchAFL,
    confirmMatch,
    getSettings,
  } from '../../lib/api.ts';
  import { episodeTypeBadge, syncStateBadge } from '../../lib/fox.ts';
  import {
    createPosterMonogram,
    POSTER_FAILED,
    POSTER_LOADING,
    resolvePosterElementState,
  } from '../../lib/poster-state.ts';
  import { resolveSonarrImageUrl } from '../../lib/sonarr-images.ts';
  import { Check, CircleAlert, CircleCheckBig, LoaderCircle, Plus, Search, Waves, X } from 'lucide-svelte';

  const START_FROM_BEGINNING: string = 'from_beginning';
  const START_FROM_CURRENT: string = 'from_current_progress';
  const ADD_STEP_SERIES: string = 'series';
  const ADD_STEP_FALLBACK_AFL: string = 'fallback_afl';
  const ADD_STEP_FALLBACK_SONARR: string = 'fallback_sonarr';
  const ADD_STEP_MATCH: string = 'match';
  const ADD_STEP_UNSAFE_MATCH: string = 'unsafe_match';
  const ADD_STEP_POST_ADD: string = 'post_add';

  interface AddOutcome {
    title: string;
    type: string;
    mappingId?: string | number | null;
  }

  interface PosterItem {
    poster?: string;
    [key: string]: unknown;
  }

  interface WizardStep {
    key: string;
    label: string;
  }

  let loading: boolean = true;
  let filter: string = '';
  let sortBy: string = 'title';

  let allSeries: SonarrSeries[] = [];
  let addSeriesLibrary: SonarrSeries[] = [];
  let seriesById: Record<string | number, SonarrSeries> = {};

  let showAddModal: boolean = false;
  let addStep: string = ADD_STEP_SERIES;
  let sonarrLoading: boolean = false;
  let seriesQuery = '';
  let selectedSonarr: SonarrSeries | null = null;
  let suggestedMatches: AFLMatch[] = [];
  let displayedMatches: AFLMatch[] = [];
  let matchLoading: boolean = false;
  let manualQuery: string = '';
  let manualSearching: boolean = false;
  let manualSearchVisible: boolean = false;
  let selectedAfl: AFLMatch | null = null;
  let confirming: boolean = false;
  let addTrackFiller: boolean = false;
  let addTrackMixed: boolean = true;
  let addTrackAnimeCanon: boolean = true;
  let addStartMode: string = START_FROM_CURRENT;
  let fallbackAflQuery: string = '';
  let cachedAflShows: AFLMatch[] = [];
  let fallbackAflResults: AFLMatch[] = [];
  let fallbackAflLoading: boolean = false;
  let selectedFallbackAfl: AFLMatch | null = null;
  let fallbackFilterTimeout: ReturnType<typeof setTimeout> | null = null;
  let sonarrLookupQuery: string = '';
  let sonarrLookupResults: SonarrSeries[] = [];
  let sonarrLookupLoading: boolean = false;
  let selectedSonarrLookup: SonarrSeries | null = null;
  let sonarrAdding: boolean = false;
  let sonarrRootFolderOptions: SelectOption[] = [];
  let sonarrQualityProfileOptions: SelectOption[] = [];
  let sonarrAddOptionsError: string = '';
  let fallbackQualityProfileId: string = '';
  let fallbackSeasonFolder: boolean = true;
  let addOutcome: AddOutcome | null = null;

  let activeWizardSteps: WizardStep[] = [];
  let currentStepIndex: number = -1;
  let mappedShows: Show[] = [];
  let filteredShows: Show[] = [];
  let availableSeries: SonarrSeries[] = [];
  let mappedSeriesIds: Set<number> = new Set();
  let filteredSeriesLibrary: SonarrSeries[] = [];

  const startModeOptions: SelectOption[] = [
    { value: START_FROM_CURRENT, label: 'Current progress' },
    { value: START_FROM_BEGINNING, label: 'Episode 1' },
  ];
  const sortOptions: SelectOption[] = [
    { value: 'title', label: 'Title (A-Z)' },
    { value: 'title_desc', label: 'Title (Z-A)' },
    { value: 'watermark', label: 'Latest progress' },
  ];
  const wizardSteps: WizardStep[] = [
    { key: ADD_STEP_SERIES, label: 'Select' },
    { key: ADD_STEP_MATCH, label: 'Match' },
    { key: ADD_STEP_UNSAFE_MATCH, label: 'Confirm' },
    { key: ADD_STEP_POST_ADD, label: 'Done' },
  ];
  const wizardFallbackSteps: WizardStep[] = [
    { key: ADD_STEP_FALLBACK_AFL, label: 'AFL Search' },
    { key: ADD_STEP_FALLBACK_SONARR, label: 'Add to Sonarr' },
    { key: ADD_STEP_MATCH, label: 'Match' },
  ];
  $: activeWizardSteps = [ADD_STEP_FALLBACK_AFL, ADD_STEP_FALLBACK_SONARR].includes(addStep) ? wizardFallbackSteps : wizardSteps;
  $: currentStepIndex = activeWizardSteps.findIndex((s: WizardStep) => s.key === addStep);
  const modalSelectContentProps: Record<string, unknown> = { class: 'z-[70] p-1' };
  const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

  $: mappedShows = ($shows || []).map((show: Show) => {
    const series = seriesById[show.sonarr_series_id] || null;
    return {
      ...show,
      poster: ($settings?.enable_anilist_artwork && show.anilist_poster_url) ? show.anilist_poster_url : posterUrl(series),
    };
  });

  $: filteredShows = mappedShows
    .filter((show: Show) => {
      if (!filter) return true;
      const query: string = filter.toLowerCase();
      return (show.sonarr_title || '').toLowerCase().includes(query) || (show.afl_title || show.afl_slug || '').toLowerCase().includes(query);
    })
    .sort((a: Show, b: Show) => {
      if (sortBy === 'title') return (a.sonarr_title || '').localeCompare(b.sonarr_title || '');
      if (sortBy === 'title_desc') return (b.sonarr_title || '').localeCompare(a.sonarr_title || '');
      if (sortBy === 'watermark') return (b.sync_watermark || 0) - (a.sync_watermark || 0);
      return 0;
    });

  $: availableSeries = allSeries
    .filter((series: SonarrSeries) => !new Set(($shows || []).map((show: Show) => show.sonarr_series_id)).has(series.id))
    .sort((a: SonarrSeries, b: SonarrSeries) => a.title.localeCompare(b.title));

  $: mappedSeriesIds = new Set(($shows || []).map((show: Show) => show.sonarr_series_id));

  $: filteredSeriesLibrary = addSeriesLibrary
    .filter((series: SonarrSeries) => {
    const query: string = seriesQuery.trim().toLowerCase();
    if (!query) return true;
    return (
      (series.title || '').toLowerCase().includes(query) ||
      String(series.year || '').includes(query)
    );
    })
    .sort((a: SonarrSeries, b: SonarrSeries) => {
      const aMapped = mappedSeriesIds.has(a.id) ? 1 : 0;
      const bMapped = mappedSeriesIds.has(b.id) ? 1 : 0;
      if (aMapped !== bMapped) return aMapped - bMapped;
      return a.title.localeCompare(b.title);
    });

  onMount(async () => {
    await loadLanding();
  });

  onDestroy(() => {
    if (typeof document !== 'undefined') {
      document.body.style.overflow = '';
    }
    if (fallbackFilterTimeout) {
      clearTimeout(fallbackFilterTimeout);
    }
  });

  async function loadLanding(): Promise<void> {
    loading = true;
    try {
      const [showsRes, seriesRes, settingsRes]: [PromiseSettledResult<Show[]>, PromiseSettledResult<SonarrSeries[]>, PromiseSettledResult<Settings>] = await Promise.allSettled([getShows(), getSonarrSeries(), getSettings()]);
      const nextShows: Show[] = showsRes.status === 'fulfilled' ? (showsRes.value || []) : [];
      shows.set(nextShows);
      allSeries = seriesRes.status === 'fulfilled' ? (seriesRes.value || []) : [];
      seriesById = Object.fromEntries(allSeries.map((series: SonarrSeries) => [series.id, series]));
      if (settingsRes.status === 'fulfilled') {
        settings.set(settingsRes.value ?? $settings);
      }
    } catch (err: any) {
      addNotification('Failed to load Mizu anime: ' + err.message, 'error');
    }
    loading = false;
  }

  function posterUrl(series: SonarrSeries | null | undefined): string {
    if (!series?.images?.length) return '';
    const preferred: SonarrImage | undefined = series.images.find((image: SonarrImage) => image.coverType === 'poster') || series.images.find((image: SonarrImage) => image.coverType === 'fanart') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, $settings?.sonarr_url || '');
  }

  function bannerUrl(show: Show, series: SonarrSeries | null | undefined): string {
    if ($settings?.enable_anilist_artwork && show.anilist_banner_url) return show.anilist_banner_url;
    if (!series?.images?.length) return '';
    const preferred: SonarrImage | undefined = series.images.find((image: SonarrImage) => image.coverType === 'fanart') || series.images.find((image: SonarrImage) => image.coverType === 'banner') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, $settings?.sonarr_url || '');
  }

  function markerChip(show: Show): string {
    return show.sync_watermark > 0 ? `#${show.sync_watermark}` : '#0';
  }

  function setPosterFrameState(node: HTMLElement, state: string): void {
    node.dataset.posterState = state;
  }

  function observePosterFrame(node: HTMLElement, show?: PosterItem): ActionReturn<PosterItem | undefined> {
    let currentShow: PosterItem | undefined = show;
    let currentImage: HTMLImageElement | null = null;

    const syncState = () => {
      setPosterFrameState(node, resolvePosterElementState({
        hasPoster: !!currentShow?.poster || !!currentImage?.getAttribute('src'),
        complete: !!currentImage?.complete,
        naturalWidth: currentImage?.naturalWidth || 0,
      }));
    };

    const handleLoad = () => setPosterFrameState(node, 'loaded');
    const handleError = () => setPosterFrameState(node, 'failed');

    const bindImage = () => {
      if (currentImage) {
        currentImage.removeEventListener('load', handleLoad);
        currentImage.removeEventListener('error', handleError);
      }

      currentImage = node.querySelector('img');

      if (!currentImage) {
        syncState();
        return;
      }

      currentImage.addEventListener('load', handleLoad);
      currentImage.addEventListener('error', handleError);
      queueMicrotask(syncState);
    };

    bindImage();

    return {
      update(nextShow: PosterItem | undefined) {
        currentShow = nextShow;
        bindImage();
      },
      destroy() {
        if (!currentImage) return;
        currentImage.removeEventListener('load', handleLoad);
        currentImage.removeEventListener('error', handleError);
      },
    };
  }

  function openShow(showId: string | number): void {
    goto(`/shows/${showId}`);
  }

  async function refreshSettings(): Promise<void> {
    try {
      const current: Settings = await getSettings();
      settings.set(current);
    } catch (err: any) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
  }

  function resetAddRuleDefaults(current: Settings = $settings): void {
    addTrackFiller = actionToTracked(current.default_filler_action, false);
    addTrackMixed = actionToTracked(current.default_mixed_canon_action, true);
    addTrackAnimeCanon = actionToTracked(current.default_anime_canon_action, true);
    addStartMode = START_FROM_CURRENT;
  }

  async function loadSonarrAddOptions(): Promise<void> {
    sonarrAddOptionsError = '';
    try {
      const data: SonarrAddOptions = await getSonarrAddOptions();
      sonarrRootFolderOptions = (data.root_folders || []).map((folder: { path: string }) => ({
        value: folder.path,
        label: folder.path,
      }));
      sonarrQualityProfileOptions = (data.quality_profiles || []).map((profile: { id: number; name: string }) => ({
        value: String(profile.id),
        label: profile.name,
      }));
      if (!fallbackQualityProfileId) {
        fallbackQualityProfileId = preferredAnimeQualityProfileId();
      }
    } catch (err: any) {
      sonarrAddOptionsError = err.message;
    }
  }

  async function refreshAddSeriesLibrary(): Promise<void> {
    addSeriesLibrary = (await getSonarrSeries(true)) || [];
  }

  function resetFallbackFlow(): void {
    fallbackAflQuery = '';
    fallbackAflResults = cachedAflShows;
    selectedFallbackAfl = null;
    sonarrLookupQuery = '';
    sonarrLookupResults = [];
    selectedSonarrLookup = null;
    fallbackQualityProfileId = '';
    fallbackSeasonFolder = true;
    addOutcome = null;
    selectedAfl = null;
  }

  async function openAddModal(): Promise<void> {
    addStep = ADD_STEP_SERIES;
    seriesQuery = '';
    selectedSonarr = null;
    suggestedMatches = [];
    displayedMatches = [];
    selectedAfl = null;
    manualQuery = '';
    manualSearchVisible = false;
    resetFallbackFlow();
    showAddModal = true;

    await Promise.all([refreshSettings(), loadSonarrAddOptions()]);
    resetAddRuleDefaults($settings);

    sonarrLoading = true;
    try {
      const [fullSeries, modalSeries]: [SonarrSeries[], SonarrSeries[]] = await Promise.all([
        allSeries.length ? Promise.resolve(allSeries) : getSonarrSeries(),
        getSonarrSeries(true),
      ]);
      allSeries = fullSeries || [];
      addSeriesLibrary = modalSeries || [];
      seriesById = Object.fromEntries(allSeries.map((series: SonarrSeries) => [series.id, series]));
    } catch (err: any) {
      addNotification('Failed to load Sonarr series: ' + err.message, 'error');
    }
    sonarrLoading = false;
  }

  function closeAddModal(): void {
    showAddModal = false;
  }

  function handleAddModalBackdropClick(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      closeAddModal();
    }
  }

  function handleWindowKeydown(event: KeyboardEvent): void {
    if (showAddModal && event.key === 'Escape') {
      closeAddModal();
    }
  }

  function openExistingMapping(seriesId: string | number): void {
    const mapped = ($shows || []).find((show: Show) => show.sonarr_series_id === seriesId);
    if (!mapped) return;
    closeAddModal();
    openShow(mapped.id);
  }

  async function goToMatchStep(): Promise<void> {
    if (!selectedSonarr) return;
    addStep = ADD_STEP_MATCH;
    manualQuery = selectedSonarr.title || '';
    await loadSuggestedMatches();
  }

  async function loadSuggestedMatches(): Promise<void> {
    if (!selectedSonarr) return;
    matchLoading = true;
    selectedAfl = null;
    try {
      suggestedMatches = await suggestAFL(selectedSonarr.id);
      displayedMatches = suggestedMatches;
      manualSearchVisible = suggestedMatches.length === 0 || !!selectedSonarr.manualMatchPending;
      const bestMatch: AFLMatch | undefined = suggestedMatches[0];
      if (bestMatch && isCompactMatchSelection(bestMatch)) {
        selectedAfl = bestMatch;
      }
      if (manualSearchVisible && selectedSonarr.manualMatchPending && manualQuery.trim()) {
        displayedMatches = await searchAFL(manualQuery.trim(), selectedSonarr.id);
      }
    } catch (err: any) {
      suggestedMatches = [];
      displayedMatches = [];
      manualSearchVisible = true;
      addNotification('Failed to load AFL suggestions: ' + err.message, 'error');
    }
    matchLoading = false;
  }

  async function handleManualSearch(): Promise<void> {
    if (!selectedSonarr || !manualSearchVisible) return;
    manualSearching = true;
    selectedAfl = null;
    try {
      displayedMatches = await searchAFL(manualQuery.trim(), selectedSonarr.id);
    } catch (err: any) {
      displayedMatches = [];
      addNotification('AFL search failed: ' + err.message, 'error');
    }
    manualSearching = false;
  }

  async function handleConfirmMatch(): Promise<void> {
    if (!selectedSonarr || !selectedAfl) return;
    confirming = true;
    try {
      const created: Show = await confirmMatch({
        sonarr_series_id: selectedSonarr.id,
        sonarr_title: selectedSonarr.title,
        afl_slug: selectedAfl.afl_show?.slug || selectedAfl.slug,
        afl_title: selectedAfl.afl_show?.title || selectedAfl.title,
        filler_action: trackedToAction(addTrackFiller),
        mixed_action: trackedToAction(addTrackMixed),
        anime_canon_action: trackedToAction(addTrackAnimeCanon),
        start_mode: addStartMode,
      });
      addNotification(`Added "${selectedSonarr.title}" to Mizu`, 'success');
      addOutcome = {
        title: selectedSonarr.title,
        type: 'mapping_created',
        mappingId: created?.id || null,
      };
      await loadLanding();
      addStep = ADD_STEP_POST_ADD;
    } catch (err: any) {
      addNotification('Failed to sync anime: ' + err.message, 'error');
    } finally {
      confirming = false;
    }
  }

  function isUnsafeMatch(match: AFLMatch): boolean {
    return !!match && match.confidence == null;
  }

  function shouldAutoCreateMappingAfterSonarrAdd(match: AFLMatch): boolean {
    return !!match && !isUnsafeMatch(match) && match.method === 'exact' && (match.confidence ?? 0) >= 1;
  }

  function isCompactMatchSelection(match: AFLMatch): boolean {
    if (!match || isUnsafeMatch(match)) return false;
    return match.method === 'exact' || match.method === 'normalized' || (match.confidence ?? 0) >= 0.95;
  }

  function preferredAnimeRootFolder(): string {
    const counts: Map<string, number> = new Map();
    let bestPath = '';
    let bestCount = 0;

    for (const series of allSeries) {
      const path: string = (series?.rootFolderPath || '').trim();
      if (!path) continue;
      const nextCount: number = (counts.get(path) || 0) + 1;
      counts.set(path, nextCount);
      if (nextCount > bestCount) {
        bestCount = nextCount;
        bestPath = path;
      }
    }

    return bestPath || sonarrRootFolderOptions[0]?.value || '';
  }

  function preferredAnimeQualityProfileId(): string {
    const counts: Map<string, number> = new Map();
    let bestValue = '';
    let bestCount = 0;

    for (const series of allSeries) {
      const value: number = Number(series?.qualityProfileId || 0);
      if (!value) continue;
      const key: string = String(value);
      const nextCount: number = (counts.get(key) || 0) + 1;
      counts.set(key, nextCount);
      if (nextCount > bestCount) {
        bestCount = nextCount;
        bestValue = key;
      }
    }

    return bestValue || sonarrQualityProfileOptions[0]?.value || '';
  }

  function ensureFallbackAddDefaults(): void {
    if (!fallbackQualityProfileId) {
      fallbackQualityProfileId = preferredAnimeQualityProfileId();
    }
    fallbackSeasonFolder = fallbackSeasonFolder ?? true;
  }

  function buildSonarrAddPayload(series: SonarrSeries): SonarrAddPayload {
    return {
      series: {
        ...series,
        seriesType: 'anime',
      },
      root_folder_path: preferredAnimeRootFolder(),
      quality_profile_id: Number(fallbackQualityProfileId || 0),
      monitor_mode: 'none',
      season_folder: fallbackSeasonFolder ?? true,
      search_on_add: false,
    };
  }

  function filterFallbackAflResults(query: string = fallbackAflQuery): void {
    const normalized = (query || '').trim().toLowerCase();
    if (!normalized) {
      fallbackAflResults = cachedAflShows;
      return;
    }

    const tokens: string[] = normalized.split(/\s+/).filter(Boolean);
    fallbackAflResults = cachedAflShows.filter((show: AFLMatch) => {
      const title = (show.title || '').toLowerCase();
      if (!title) return false;
      if (title.includes(normalized)) return true;
      return tokens.every((token) => title.includes(token));
    });
  }

  function scheduleFallbackAflFilter(): void {
    if (fallbackFilterTimeout) {
      clearTimeout(fallbackFilterTimeout);
    }
    fallbackFilterTimeout = setTimeout(() => {
      filterFallbackAflResults();
    }, 120);
  }

  async function ensureCachedAflShowsLoaded(): Promise<void> {
    if (cachedAflShows.length > 0) {
      fallbackAflResults = cachedAflShows;
      return;
    }

    fallbackAflLoading = true;
    selectedFallbackAfl = null;
    try {
      cachedAflShows = await searchAFL('');
      fallbackAflResults = cachedAflShows;
    } catch (err: any) {
      cachedAflShows = [];
      fallbackAflResults = [];
      addNotification('AFL cache load failed: ' + err.message, 'error');
    } finally {
      fallbackAflLoading = false;
    }
  }

  async function goToFallbackAflStep(prefill: string = ''): Promise<void> {
    addStep = ADD_STEP_FALLBACK_AFL;
    resetFallbackFlow();
    fallbackAflQuery = prefill;
    await ensureCachedAflShowsLoaded();
    filterFallbackAflResults(prefill);
  }

  async function loadSonarrLookupResults(): Promise<void> {
    if (!sonarrLookupQuery.trim()) return;
    sonarrLookupLoading = true;
    selectedSonarrLookup = null;
    try {
      sonarrLookupResults = await lookupSonarrSeries(sonarrLookupQuery.trim());
    } catch (err: any) {
      sonarrLookupResults = [];
      addNotification('Sonarr lookup failed: ' + err.message, 'error');
    }
    sonarrLookupLoading = false;
  }

  async function goToFallbackSonarrStep(): Promise<void> {
    if (!selectedFallbackAfl) return;
    sonarrLookupQuery = selectedFallbackAfl.title || '';
    ensureFallbackAddDefaults();
    addStep = ADD_STEP_FALLBACK_SONARR;
    await loadSonarrLookupResults();
  }

  function upsertSeriesLocally(series: SonarrSeries): void {
    allSeries = [...allSeries.filter((item: SonarrSeries) => item.id !== series.id), series].sort((a: SonarrSeries, b: SonarrSeries) => a.title.localeCompare(b.title));
    seriesById = Object.fromEntries(allSeries.map((item: SonarrSeries) => [item.id, item]));
  }

  async function handleKeepSonarrOnly(): Promise<void> {
    if (!selectedSonarr?.id) return;
    try {
      await markManualMatchPending(selectedSonarr.id);
      await refreshAddSeriesLibrary();
      addOutcome = {
        title: selectedSonarr.title,
        type: 'pending_manual_match',
      };
      selectedSonarr = { ...selectedSonarr, manualMatchPending: true };
      addStep = ADD_STEP_POST_ADD;
    } catch (err: any) {
      addNotification('Failed to persist manual match reminder: ' + err.message, 'error');
    }
  }

  async function handleAddSelectedSonarrLookup(): Promise<void> {
    ensureFallbackAddDefaults();
    if (!selectedFallbackAfl || !selectedSonarrLookup) return;
    const fallbackAflSlug = selectedFallbackAfl.slug;
    sonarrAdding = true;
    try {
      const created: SonarrSeries = await addSonarrSeries(buildSonarrAddPayload(selectedSonarrLookup));
      upsertSeriesLocally(created);
      selectedSonarr = created;
      manualQuery = selectedFallbackAfl.title || created.title || '';

      const results: AFLMatch[] = await searchAFL(manualQuery.trim(), created.id);
      suggestedMatches = results;
      displayedMatches = results;
      manualSearchVisible = results.length === 0 || !!created.manualMatchPending;
      const chosenResult: AFLMatch | undefined = results.find((item: AFLMatch) => matchSlug(item) === fallbackAflSlug);
      if (chosenResult && shouldAutoCreateMappingAfterSonarrAdd(chosenResult)) {
        selectedAfl = chosenResult;
        await handleConfirmMatch();
        return;
      }

      if (chosenResult && !isUnsafeMatch(chosenResult)) {
        selectedAfl = chosenResult;
        addStep = ADD_STEP_MATCH;
        return;
      }

      if (chosenResult) {
        selectedAfl = chosenResult;
        addStep = ADD_STEP_UNSAFE_MATCH;
        return;
      }

      await handleKeepSonarrOnly();
    } catch (err: any) {
      addNotification('Failed to add anime to Sonarr: ' + err.message, 'error');
    } finally {
      sonarrAdding = false;
    }
  }

  async function handleConfirmUnsafeOverride(): Promise<void> {
    await handleConfirmMatch();
  }

  async function handlePrimaryMatchAction(): Promise<void> {
    if (!selectedAfl) return;
    if (isUnsafeMatch(selectedAfl)) {
      addStep = ADD_STEP_UNSAFE_MATCH;
      return;
    }
    await handleConfirmMatch();
  }

  function returnToSeriesStep(prefill: string = ''): void {
    addStep = ADD_STEP_SERIES;
    addOutcome = null;
    selectedAfl = null;
    selectedFallbackAfl = null;
    selectedSonarrLookup = null;
    sonarrLookupResults = [];
    fallbackAflResults = [];
    if (prefill) {
      seriesQuery = prefill;
    }
  }

  function matchTitle(match: AFLMatch | null | undefined): string {
    return match?.afl_show?.title || match?.title || '';
  }

  function matchSlug(match: AFLMatch | null | undefined): string {
    return match?.afl_show?.slug || match?.slug || '';
  }

  function optionState(checked: boolean): string {
    return checked ? 'Included' : 'Skipped';
  }

  function isActionTracked(action: string | undefined): boolean {
    return action === 'monitor';
  }

  function showRulePips(show: Show): { key: string; label: string; tracked: boolean; color: string }[] {
    return [
      { key: 'canon', label: 'canon', tracked: isActionTracked(show.anime_canon_action), color: '#74c0fc' },
      { key: 'mixed', label: 'mixed', tracked: isActionTracked(show.mixed_action), color: '#ffa94d' },
      { key: 'filler', label: 'filler', tracked: isActionTracked(show.filler_action), color: '#ffa8a8' },
    ];
  }

  function showSyncLampClass(enabled: boolean): string {
    return enabled ? 'lamp' : 'lamp lamp-warn';
  }

  $: if (typeof document !== 'undefined') {
    document.body.style.overflow = showAddModal ? 'hidden' : '';
  }
</script>

<svelte:head>
  <title>Mizu - Anime</title>
</svelte:head>

<div class="space-y-6">
  <section class="flex flex-wrap items-end gap-x-4 gap-y-3">
    <div class="space-y-1">
      <h1 class="text-2xl font-bold tracking-tight text-foreground">Anime</h1>
      <p class="text-sm text-muted-foreground">
        {#if !loading}
          <span class="font-mono tabular-nums text-foreground">{($shows || []).length}</span> tracked ·
        {/if}
        Tracked series, sync status, and filler mapping.
      </p>
    </div>
    <div class="ml-auto">
      <Button onclick={openAddModal}>
        <Plus size={16} />
        Add anime
      </Button>
    </div>
  </section>

  <section class="flex flex-col overflow-hidden rounded-lg glass-inset sm:flex-row sm:items-stretch">
    <div class="relative min-w-0 flex-1">
      <span class="pointer-events-none absolute inset-y-0 left-0 z-10 flex items-center pl-3.5 text-[#868e96]">
        <Search size={16} />
      </span>
      <Input
        type="text"
        placeholder="Search anime"
        bind:value={filter}
        class="h-10 w-full rounded-none border-0! bg-transparent! pl-10 shadow-none! focus-visible:border-transparent focus-visible:ring-0"
        aria-label="Search anime"
      />
    </div>
    <div class="border-t border-rule sm:w-48 sm:border-l sm:border-t-0">
      <Select.Root type="single" bind:value={sortBy}>
        <Select.Trigger class="h-10 w-full rounded-none border-0! bg-transparent! font-mono text-xs shadow-none! focus-visible:border-transparent focus-visible:ring-0">
          {#if sortBy}
            {@const selected = sortOptions.find(i => i.value === sortBy)}
            {selected?.label || 'Sort by'}
          {:else}
            <span class="text-muted-foreground">Sort by</span>
          {/if}
        </Select.Trigger>
        <Select.Content>
          {#each sortOptions as item}
            <Select.Item value={item.value} label={item.label} />
          {/each}
        </Select.Content>
      </Select.Root>
    </div>
  </section>

  {#if loading}
    <div class="flex min-h-56 flex-col items-center justify-center gap-3 rounded-lg glass-surface text-center">
      <LoaderCircle size={26} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-sm font-semibold text-foreground">Loading anime</h2>
        <p class="text-sm text-muted-foreground">Loading mappings and Sonarr state.</p>
      </div>
    </div>
  {:else if filteredShows.length === 0 && ($shows || []).length === 0}
    <div class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-lg glass-surface px-6 text-center">
      <span class="flex size-10 items-center justify-center rounded-lg border border-rule glass-inset text-muted-foreground">
        <Waves size={18} />
      </span>
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">No anime synced yet</h2>
        <p class="max-w-md text-sm leading-6 text-muted-foreground">Add your first Sonarr anime mapping to start monitoring filler decisions in Mizu.</p>
      </div>
      <Button onclick={openAddModal}>
        <Plus size={16} />
        Add anime
      </Button>
    </div>
  {:else if filteredShows.length === 0}
    <div class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-lg glass-surface px-6 text-center">
      <span class="flex size-10 items-center justify-center rounded-lg border border-rule glass-inset text-muted-foreground">
        <CircleAlert size={18} />
      </span>
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">No matching anime</h2>
        <p class="text-sm text-muted-foreground">Try another title, slug, or keyword.</p>
      </div>
    </div>
  {:else}
    <div class="rule-label"><span>Tracked</span></div>
    <section class="overflow-hidden rounded-lg border border-rule bg-card" use:autoAnimate>
      {#each filteredShows as show}
        {@const series = seriesById[show.sonarr_series_id]}
        {@const episodeCount = series?.statistics?.episodeCount}
        {@const banner = bannerUrl(show, series)}
        <a
          href={`/shows/${show.id}`}
          class="group relative flex h-16 items-center gap-3.5 overflow-hidden border-b border-[#10141b] px-4 transition-colors last:border-b-0 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary sm:gap-4 sm:px-5"
          onclick={(event) => {
            event.preventDefault();
            openShow(show.id);
          }}
          aria-label={`Open ${show.sonarr_title}`}
        >
          {#if banner}
            <div class="absolute inset-0 z-0" aria-hidden="true">
              <img
                src={banner}
                alt=""
                loading="lazy"
                decoding="async"
                class="h-full w-full object-cover opacity-[0.18] transition-all duration-500 ease-out group-hover:scale-105 group-hover:opacity-[0.28]"
              />
              <div class="absolute inset-0 bg-gradient-to-r from-background via-background/85 to-background/40"></div>
            </div>
          {/if}

          <div class="relative z-10 min-w-0 flex-1">
            <h2 class="line-clamp-1 text-sm font-semibold text-foreground">{show.sonarr_title}</h2>
            <p class="mt-0.5 font-mono text-[11px] tabular-nums text-[#868e96]">{markerChip(show)}</p>
          </div>

          <span class={`relative z-10 shrink-0 rounded-full border px-2.5 py-1 font-mono text-[10px] uppercase tracking-[0.08em] ${show.enabled ? 'border-[#12b886]/30 bg-[rgba(18,184,134,.12)] text-[#63e6be]' : 'border-rule bg-white/[0.04] text-[#adb5bd]'}`}>
            {show.enabled ? 'Syncing' : 'Paused'}
          </span>
        </a>
      {/each}
    </section>
  {/if}
</div>

<svelte:window onkeydown={handleWindowKeydown} />

{#if showAddModal}
  <div
    class="show-add-modal-shell fixed inset-0 z-50 grid place-items-center bg-background/80 p-3 sm:p-6"
    role="presentation"
    onclick={handleAddModalBackdropClick}
    transition:fade={{ duration: 180 }}
  >
    <div
      class="show-add-modal-panel relative flex max-h-[calc(100vh-1.5rem)] w-full max-w-3xl flex-col overflow-hidden rounded-xl glass-overlay bg-popover sm:max-h-[calc(100vh-3rem)]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="show-add-modal-title"
      tabindex="-1"
      onclick={(event) => event.stopPropagation()}
      onkeydown={(event) => event.stopPropagation()}
      transition:scale={{ duration: 220, start: 0.96 }}
    >
      <!-- Ledger header -->
      <div class="shrink-0 space-y-3 border-b border-rule px-5 pb-4 pt-5 sm:px-7">
        <div class="flex items-center gap-3">
          <h2 id="show-add-modal-title" class="text-xl font-semibold tracking-tight text-foreground">Add anime</h2>
          <button
            type="button"
            class="show-add-modal-close ml-auto inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-rule bg-card text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            onclick={closeAddModal}
            aria-label="Close add anime modal"
          >
            <X size={15} />
          </button>
        </div>

        <!-- Mono step progress -->
        <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1 font-mono text-[10px] uppercase tracking-[0.1em]" aria-hidden="true">
          {#each activeWizardSteps as step, i}
            {@const isCurrent = step.key === addStep}
            {@const isPast = currentStepIndex > i}
            {#if i > 0}
              <span class="h-px w-4 bg-rule"></span>
            {/if}
            <span class="flex items-center gap-1.5 {isCurrent ? 'text-foreground' : isPast ? 'text-muted-foreground' : 'text-[#868e96]'}">
              {#if isPast}
                <Check size={10} class="text-primary" />
              {/if}
              {step.label}
            </span>
          {/each}
        </div>
        <div class="h-px w-full bg-rule" aria-hidden="true"></div>

        <p class="text-sm leading-6 text-muted-foreground">
          {#if addStep === ADD_STEP_SERIES}
            Search your actionable Sonarr anime library first. If the anime is missing, switch to cached AFL search and add it into Sonarr from here.
          {:else if addStep === ADD_STEP_FALLBACK_AFL}
            Search cached AnimeFillerList titles, then use the chosen title to look up the right Sonarr series before adding it.
          {:else if addStep === ADD_STEP_FALLBACK_SONARR}
            Confirm the Sonarr series Mizu should add for <span class="font-medium text-muted-foreground">{selectedFallbackAfl?.title}</span>.
          {:else if addStep === ADD_STEP_POST_ADD}
            {#if addOutcome?.type === 'mapping_created'}
              <span class="font-medium text-muted-foreground">{addOutcome?.title}</span> is now tracked by Mizu.
            {:else}
              <span class="font-medium text-muted-foreground">{addOutcome?.title}</span> is in Sonarr now, but still needs a safe AFL mapping before Mizu can track it.
            {/if}
          {:else if addStep === ADD_STEP_UNSAFE_MATCH}
            Mizu found a possible cached AFL entry for <span class="font-medium text-muted-foreground">{selectedSonarr?.title}</span>, but it needs your confirmation before creating the mapping.
          {:else}
            Match <span class="font-medium text-muted-foreground">{selectedSonarr?.title}</span> with the correct AnimeFillerList entry.
          {/if}
        </p>
      </div>

      <!-- Scrollable body -->
      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-5 sm:px-7 sm:py-6">
        {#if addStep === ADD_STEP_SERIES}
          {#if sonarrLoading}
            <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
              <LoaderCircle size={28} class="animate-spin text-primary" />
              <p class="text-sm text-muted-foreground">Loading available Sonarr anime.</p>
            </div>
          {:else if allSeries.length === 0}
            <div class="rounded-lg glass-inset p-6 text-center">
              <h3 class="text-sm font-semibold text-foreground">No anime found in Sonarr</h3>
              <p class="mt-2 text-sm leading-6 text-muted-foreground">Sonarr returned no anime series for this library.</p>
            </div>
          {:else}
            <div class="space-y-4">
              <div class="space-y-2">
                <Label for="show-add-series-search">Search</Label>
                <div class="relative">
                  <div class="pointer-events-none absolute inset-y-0 left-3 z-10 flex items-center text-muted-foreground">
                    <Search size={16} />
                  </div>
                  <Input
                    id="show-add-series-search"
                    type="text"
                    placeholder="Search Sonarr anime"
                    class="pl-10"
                    bind:value={seriesQuery}
                  />
                </div>
              </div>

              {#if filteredSeriesLibrary.length === 0}
                <div class="space-y-4 rounded-lg glass-inset p-6 text-center">
                  <h3 class="text-sm font-semibold text-foreground">No anime match that search</h3>
                  <p class="mt-2 text-sm leading-6 text-muted-foreground">Try a different title, year, or jump into the cached AFL list and add it from there.</p>
                  <div class="flex justify-center">
                    <Button variant="secondary" onclick={() => goToFallbackAflStep(seriesQuery.trim())}>
                      <Search size={16} />
                      Not in Sonarr? Search AFL and add to Sonarr
                    </Button>
                  </div>
                </div>
              {:else}
                <ScrollArea class="h-[23rem] rounded-lg border border-rule bg-card">
                  <div class="divide-y divide-[#10141b]">
                    {#each filteredSeriesLibrary as series}
                      {@const seriesPoster = posterUrl(series)}
                      {@const seriesMapped = mappedSeriesIds.has(series.id)}
                      {@const manualPending = !!series.manualMatchPending}
                      {@const isSelected = selectedSonarr?.id === series.id}
                      <button
                        type="button"
                        class="group flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary/10' : seriesMapped ? 'opacity-60 hover:bg-white/[0.02]' : 'hover:bg-white/[0.02]'}"
                        onclick={() => {
                          if (seriesMapped) {
                            openExistingMapping(series.id);
                            return;
                          }
                          selectedSonarr = series;
                        }}
                        aria-pressed={isSelected}
                      >
                        <div
                          class="poster-frame poster-thumb relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-[5px] border border-rule bg-card sm:w-11"
                          data-poster-state={seriesPoster ? POSTER_LOADING : POSTER_FAILED}
                          use:observePosterFrame={series}
                        >
                          <div class="relative aspect-[2/3] h-full w-full overflow-hidden">
                            {#if seriesPoster}
                              <div class="poster-skeleton absolute inset-0" aria-hidden="true">
                                <div class="poster-skeleton__shine"></div>
                              </div>
                              <img
                                src={seriesPoster}
                                alt={series.title}
                                width="160"
                                height="240"
                                loading="lazy"
                                decoding="async"
                                class="poster-image h-full w-full object-cover opacity-0 transition-all duration-500 ease-in-out"
                              />
                            {/if}
                          </div>
                          <div class="poster-fallback bg-card text-sm font-semibold tracking-tight text-muted-foreground">
                            {createPosterMonogram(series.title)}
                          </div>
                        </div>

                        <div class="min-w-0 flex-1 space-y-0.5">
                          <div class="line-clamp-1 text-sm font-medium text-foreground">{series.title}</div>
                          <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[11px] tabular-nums text-[#868e96]">
                            <span>{series.year || 'Anime'}</span>
                            {#if series.statistics?.episodeCount}
                              <span aria-hidden="true">·</span>
                              <span>{series.statistics.episodeCount} eps</span>
                            {/if}
                            {#if manualPending}
                              <span aria-hidden="true">·</span>
                              <span class="text-[#fd7e14]">Needs match</span>
                            {:else if seriesMapped}
                              <span aria-hidden="true">·</span>
                              <span>Already added</span>
                            {/if}
                          </div>
                        </div>

                        {#if seriesMapped}
                          <span class="shrink-0 font-mono text-[10px] uppercase tracking-[0.1em] text-muted-foreground">Open</span>
                        {:else if manualPending}
                          <span class="shrink-0 font-mono text-[10px] uppercase tracking-[0.1em] text-[#fd7e14]">Match</span>
                        {:else if isSelected}
                          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                            <Check size={13} />
                          </div>
                        {:else}
                          <div class="h-6 w-6 shrink-0 rounded-full border border-rule group-hover:border-primary/40"></div>
                        {/if}
                      </button>
                    {/each}
                  </div>
                </ScrollArea>
              {/if}

            </div>
          {/if}
        {:else if addStep === ADD_STEP_FALLBACK_AFL}
          <div class="space-y-4">
            <div class="space-y-2">
              <Label for="fallback-afl-search">Cached AFL title</Label>
              <Input
                id="fallback-afl-search"
                type="text"
                placeholder="Filter cached AnimeFillerList titles"
                bind:value={fallbackAflQuery}
                oninput={scheduleFallbackAflFilter}
              />
            </div>

            {#if fallbackAflLoading}
              <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
                <LoaderCircle size={28} class="animate-spin text-primary" />
                <p class="text-sm text-muted-foreground">Loading cached AnimeFillerList titles.</p>
              </div>
            {:else if fallbackAflResults.length === 0}
              <div class="rounded-lg glass-inset p-6 text-center">
                <h3 class="text-sm font-semibold text-foreground">No cached AFL titles found</h3>
                <p class="mt-2 text-sm leading-6 text-muted-foreground">Try another filter or refresh the AFL cache in Settings if this anime is new.</p>
              </div>
            {:else}
              <ScrollArea class="h-[22rem] rounded-lg border border-rule bg-card">
                <div class="divide-y divide-[#10141b]">
                  {#each fallbackAflResults as result}
                    {@const isSelected = selectedFallbackAfl?.slug === result.slug}
                    <button
                      type="button"
                      class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary/10' : 'hover:bg-white/[0.02]'}"
                      onclick={() => (selectedFallbackAfl = result)}
                    >
                      <div class="min-w-0">
                        <div class="truncate text-sm font-medium text-foreground">{result.title}</div>
                      </div>
                      {#if isSelected}
                        <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                          <Check size={13} />
                        </div>
                      {:else}
                        <div class="h-6 w-6 shrink-0 rounded-full border border-rule"></div>
                      {/if}
                    </button>
                  {/each}
                </div>
              </ScrollArea>
            {/if}
          </div>
        {:else if addStep === ADD_STEP_FALLBACK_SONARR}
          <div class="space-y-4">
            <div class="space-y-2">
              <Label for="fallback-sonarr-search">Sonarr lookup</Label>
              <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                <Input
                  id="fallback-sonarr-search"
                  type="text"
                  placeholder="Search Sonarr for the selected AFL anime"
                  bind:value={sonarrLookupQuery}
                  onkeydown={(event) => event.key === 'Enter' && loadSonarrLookupResults()}
                />
                <Button variant="secondary" onclick={loadSonarrLookupResults} disabled={sonarrLookupLoading || !sonarrLookupQuery.trim()}>
                  {#if sonarrLookupLoading}
                    <LoaderCircle size={16} class="animate-spin" />
                  {:else}
                    <Search size={16} />
                  {/if}
                  Search
                </Button>
              </div>
            </div>

            {#if sonarrLookupLoading}
              <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
                <LoaderCircle size={28} class="animate-spin text-primary" />
                <p class="text-sm text-muted-foreground">Looking up Sonarr candidates.</p>
              </div>
            {:else if sonarrLookupResults.length === 0}
              <div class="rounded-lg glass-inset p-6 text-center">
                <h3 class="text-sm font-semibold text-foreground">No Sonarr results found</h3>
                <p class="mt-2 text-sm leading-6 text-muted-foreground">Try another title variation for {selectedFallbackAfl?.title} and search again.</p>
              </div>
            {:else}
              <ScrollArea class="h-[18rem] rounded-lg border border-rule bg-card">
                <div class="divide-y divide-[#10141b]">
                  {#each sonarrLookupResults as result}
                    {@const lookupPoster = posterUrl(result)}
                    {@const isSelected = selectedSonarrLookup?.tvdbId === result.tvdbId && selectedSonarrLookup?.title === result.title}
                    <button
                      type="button"
                      class="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary/10' : 'hover:bg-white/[0.02]'}"
                      onclick={() => (selectedSonarrLookup = result)}
                    >
                      <div
                        class="poster-frame poster-thumb relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-[5px] border border-rule bg-card sm:w-11"
                        data-poster-state={lookupPoster ? POSTER_LOADING : POSTER_FAILED}
                        use:observePosterFrame={result}
                      >
                        <div class="relative aspect-[2/3] h-full w-full overflow-hidden">
                          {#if lookupPoster}
                            <div class="poster-skeleton absolute inset-0" aria-hidden="true">
                              <div class="poster-skeleton__shine"></div>
                            </div>
                            <img
                              src={lookupPoster}
                              alt={result.title}
                              width="160"
                              height="240"
                              loading="lazy"
                              decoding="async"
                              class="poster-image h-full w-full object-cover opacity-0 transition-all duration-500 ease-in-out"
                            />
                          {/if}
                        </div>
                        <div class="poster-fallback bg-card text-sm font-semibold tracking-tight text-muted-foreground">
                          {createPosterMonogram(result.title)}
                        </div>
                      </div>

                      <div class="min-w-0 flex-1 space-y-0.5">
                        <div class="truncate text-sm font-medium text-foreground">{result.title}</div>
                        <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[11px] text-[#868e96]">
                          <span>{result.year || 'Unknown year'}</span>
                          {#if result.network}
                            <span aria-hidden="true">·</span>
                            <span>{result.network}</span>
                          {/if}
                          {#if result.status}
                            <span aria-hidden="true">·</span>
                            <span>{result.status}</span>
                          {/if}
                        </div>
                      </div>

                      {#if isSelected}
                        <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                          <Check size={13} />
                        </div>
                      {:else}
                        <div class="h-6 w-6 shrink-0 rounded-full border border-rule"></div>
                      {/if}
                    </button>
                  {/each}
                </div>
              </ScrollArea>
            {/if}

            <div class="grid gap-3 rounded-lg border border-rule bg-card p-4 md:grid-cols-[minmax(0,1fr)_18rem] md:items-start">
              <div class="space-y-2">
                <Label for="fallback-quality-profile">Quality profile</Label>
                <div class="max-w-xs">
                  <Select.Root type="single" bind:value={fallbackQualityProfileId}>
                    <Select.Trigger id="fallback-quality-profile" class="w-full">
                      {#if fallbackQualityProfileId}
                        {@const selected = sonarrQualityProfileOptions.find(i => i.value === fallbackQualityProfileId)}
                        {selected?.label || 'Choose a quality profile'}
                      {:else}
                        <span class="text-muted-foreground">Choose a quality profile</span>
                      {/if}
                    </Select.Trigger>
                    <Select.Content>
                      {#each sonarrQualityProfileOptions as item}
                        <Select.Item value={item.value} label={item.label} />
                      {/each}
                    </Select.Content>
                  </Select.Root>
                </div>
              </div>

              <label class="flex items-center justify-between gap-4 rounded-lg glass-inset px-4 py-3">
                <div class="text-sm font-medium text-foreground">Season folder</div>
                <Checkbox bind:checked={fallbackSeasonFolder} />
              </label>

              {#if sonarrAddOptionsError}
                <div class="rounded-lg border border-[#fd7e14]/30 bg-[#fd7e14]/10 px-4 py-3 text-sm text-[#fd7e14]">
                  Unable to load Sonarr quality profiles right now: {sonarrAddOptionsError}
                </div>
              {/if}
            </div>
          </div>
        {:else if addStep === ADD_STEP_UNSAFE_MATCH}
          <div class="space-y-4 rounded-lg border border-[#fd7e14]/30 bg-[#fd7e14]/10 p-5">
            <div class="flex items-start gap-3">
              <span class="mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-[#fd7e14]/30 text-[#fd7e14]">
                <CircleAlert size={15} />
              </span>
              <div class="space-y-2">
                <h3 class="text-sm font-semibold text-foreground">This cached AFL match needs confirmation</h3>
                <p class="text-sm leading-6 text-[#fd7e14]/90">
                  Mizu could not safely verify that <span class="font-medium text-foreground">{matchTitle(selectedAfl)}</span> is the right AnimeFillerList entry for
                  <span class="font-medium text-foreground"> {selectedSonarr?.title}</span>. You can still create the mapping manually, or keep the series in Sonarr only and match it later.
                </p>
              </div>
            </div>

            <div class="rounded-lg glass-inset p-4">
              <div class="text-sm font-semibold text-foreground">{matchTitle(selectedAfl)}</div>
              <div class="mt-1 font-mono text-xs text-muted-foreground">{matchSlug(selectedAfl)}</div>
            </div>
          </div>
        {:else if addStep === ADD_STEP_POST_ADD}
          <div class="space-y-4 rounded-lg border border-rule bg-card p-5">
            <div class="flex items-start gap-3">
              <span class="mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-[#12b886]/30 text-[#12b886]">
                <CircleCheckBig size={15} />
              </span>
              <div class="space-y-2">
                {#if addOutcome?.type === 'mapping_created'}
                  <h3 class="text-sm font-semibold text-foreground">Mapping created</h3>
                  <p class="text-sm leading-6 text-muted-foreground">
                    <span class="font-medium text-foreground">{addOutcome?.title}</span> is now tracked by Mizu. You can configure its tracking rules and episode monitoring on the detail page.
                  </p>
                {:else}
                  <h3 class="text-sm font-semibold text-foreground">Added to Sonarr</h3>
                  <p class="text-sm leading-6 text-muted-foreground">
                    {addOutcome?.title || selectedSonarr?.title} is in Sonarr now. Mizu did not create a mapping because it could not confirm a safe AFL match, so this anime will stay visible in the add modal as <span class="font-medium text-foreground">Needs match</span>.
                  </p>
                {/if}
              </div>
            </div>
          </div>
        {:else}
          {#if manualSearchVisible}
            <div class="space-y-3 rounded-lg border border-rule bg-card p-4">
              <div class="space-y-1">
                <Label for="afl-search">Manual search</Label>
                <p class="text-sm text-muted-foreground">Automatic matching was not strong enough, so you can search the cached AnimeFillerList titles directly.</p>
              </div>
              <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                <Input
                  id="afl-search"
                  type="text"
                  placeholder="Search cached AnimeFillerList titles"
                  bind:value={manualQuery}
                  onkeydown={(event) => event.key === 'Enter' && handleManualSearch()}
                />
                <Button variant="secondary" onclick={handleManualSearch} disabled={manualSearching}>
                  {#if manualSearching}
                    <LoaderCircle size={16} class="animate-spin" />
                  {:else}
                    <Search size={16} />
                  {/if}
                  Search
                </Button>
              </div>
            </div>
          {/if}

          {#if matchLoading}
            <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
              <LoaderCircle size={28} class="animate-spin text-primary" />
              <p class="text-sm text-muted-foreground">Looking for strong cached AnimeFillerList matches.</p>
            </div>
          {:else if displayedMatches.length === 0}
            <div class="rounded-lg glass-inset p-6 text-center">
              <h3 class="text-sm font-semibold text-foreground">No matches found</h3>
              <p class="mt-2 text-sm leading-6 text-muted-foreground">Refresh the AnimeFillerList cache in Settings if this anime is new, then try again.</p>
            </div>
          {:else if selectedAfl && isCompactMatchSelection(selectedAfl)}
            <div class="space-y-4 rounded-lg border border-rule bg-card p-5">
              <div class="space-y-2">
                <div class="text-sm font-semibold text-foreground">{matchTitle(selectedAfl)}</div>
                <div class="flex flex-wrap gap-2">
                  <span class="font-mono text-[10px] uppercase tracking-[0.1em] rounded-[5px] border border-[#74c0fc]/30 bg-[#74c0fc]/10 px-2 py-[3px] text-[#74c0fc]">{Math.round((selectedAfl.confidence || 0) * 100)}% confidence</span>
                  <span class="font-mono text-[10px] uppercase tracking-[0.1em] rounded-[5px] border border-rule bg-card px-2 py-[3px] text-muted-foreground">{selectedAfl.method}</span>
                </div>
                <p class="text-sm leading-6 text-muted-foreground">Looks like an exact cached AFL match, so this step is just a quick confirmation before Mizu creates the mapping.</p>
              </div>
            </div>
          {:else}
            <ScrollArea class="h-[18rem] rounded-lg border border-rule bg-card">
              <div class="divide-y divide-[#10141b]">
                {#each displayedMatches as result}
                  {@const isSelected = selectedAfl && matchSlug(selectedAfl) === matchSlug(result)}
                  <button
                    type="button"
                    class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary/10' : 'hover:bg-white/[0.02]'}"
                    onclick={() => (selectedAfl = result)}
                  >
                    <div class="min-w-0 space-y-1">
                      <div class="truncate text-sm font-medium text-foreground">{matchTitle(result)}</div>
                      <div class="truncate font-mono text-xs text-muted-foreground">{matchSlug(result)}</div>
                      <div class="flex flex-wrap gap-2">
                        {#if result.confidence != null}
                          <span class="font-mono text-[10px] uppercase tracking-[0.1em] rounded-[5px] border border-[#74c0fc]/30 bg-[#74c0fc]/10 px-2 py-[3px] text-[#74c0fc]">{Math.round(result.confidence * 100)}% confidence</span>
                          <span class="font-mono text-[10px] uppercase tracking-[0.1em] rounded-[5px] border border-rule bg-card px-2 py-[3px] text-muted-foreground">{result.method}</span>
                        {:else}
                          <span class="font-mono text-[10px] uppercase tracking-[0.1em] rounded-[5px] border border-[#fd7e14]/30 bg-[#fd7e14]/10 px-2 py-[3px] text-[#fd7e14]">Manual review</span>
                        {/if}
                      </div>
                    </div>
                    {#if isSelected}
                      <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                        <Check size={13} />
                      </div>
                    {:else}
                      <div class="h-6 w-6 shrink-0 rounded-full border border-rule"></div>
                    {/if}
                  </button>
                {/each}
              </div>
            </ScrollArea>
          {/if}

          {#if selectedAfl}
            <div class="space-y-4 rounded-lg border border-rule bg-card p-4">
              <div class="space-y-1">
                <h3 class="text-sm font-semibold text-foreground">Default tracking rules</h3>
                <p class="text-sm text-muted-foreground">Set the initial monitoring rules for this mapping before it is created.</p>
              </div>

              <div class="grid gap-3 md:grid-cols-3">
                <label class="flex items-start gap-3 rounded-lg glass-inset p-4">
                  <Checkbox bind:checked={addTrackAnimeCanon} />
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-foreground">Anime canon</div>
                    <span class="inline-block font-mono text-[10px] uppercase tracking-[0.1em] {addTrackAnimeCanon ? 'text-[#74c0fc]' : 'text-[#868e96]'}">{optionState(addTrackAnimeCanon)}</span>
                  </div>
                </label>

                <label class="flex items-start gap-3 rounded-lg glass-inset p-4">
                  <Checkbox bind:checked={addTrackMixed} />
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-foreground">Mixed canon</div>
                    <span class="inline-block font-mono text-[10px] uppercase tracking-[0.1em] {addTrackMixed ? 'text-[#ffa94d]' : 'text-[#868e96]'}">{optionState(addTrackMixed)}</span>
                  </div>
                </label>

                <label class="flex items-start gap-3 rounded-lg glass-inset p-4">
                  <Checkbox bind:checked={addTrackFiller} />
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-foreground">Filler</div>
                    <span class="inline-block font-mono text-[10px] uppercase tracking-[0.1em] {addTrackFiller ? 'text-[#ffa8a8]' : 'text-[#868e96]'}">{optionState(addTrackFiller)}</span>
                  </div>
                </label>
              </div>

              <div class="space-y-2">
                <Label for="start-mode">Start tracking from</Label>
                <Select.Root type="single" bind:value={addStartMode}>
                  <Select.Trigger id="start-mode" class="w-full">
                    {#if addStartMode}
                      {@const selected = startModeOptions.find(i => i.value === addStartMode)}
                      {selected?.label || 'Choose a start point'}
                    {:else}
                      <span class="text-muted-foreground">Choose a start point</span>
                    {/if}
                  </Select.Trigger>
                  <Select.Content>
                    {#each startModeOptions as item}
                      <Select.Item value={item.value} label={item.label} />
                    {/each}
                  </Select.Content>
                </Select.Root>
                <p class="text-sm text-muted-foreground">Current progress uses Sonarr’s latest monitored episode. Episode 1 starts from the beginning.</p>
              </div>
            </div>
          {/if}
        {/if}

      <!-- Footer with actions -->
        <div class="flex shrink-0 flex-col-reverse gap-3 border-t border-rule px-5 py-4 sm:flex-row sm:justify-between sm:px-7">
          <div class="flex flex-col-reverse gap-3 sm:flex-row">
            {#if addStep !== ADD_STEP_POST_ADD}
              <Button variant="secondary" onclick={closeAddModal}>Cancel</Button>
            {/if}
            {#if addStep === ADD_STEP_FALLBACK_AFL}
              <Button variant="ghost" onclick={() => returnToSeriesStep()}>Back to Sonarr</Button>
            {:else if addStep === ADD_STEP_FALLBACK_SONARR}
              <Button variant="ghost" onclick={() => (addStep = ADD_STEP_FALLBACK_AFL)}>Back to AFL</Button>
            {:else if addStep === ADD_STEP_MATCH}
              <Button variant="ghost" onclick={() => returnToSeriesStep(selectedSonarr?.title || '')}>Change anime</Button>
            {:else if addStep === ADD_STEP_UNSAFE_MATCH}
              <Button variant="ghost" onclick={() => (addStep = ADD_STEP_MATCH)}>Back to matches</Button>
            {/if}
          </div>

          <div class="flex flex-col gap-3 sm:flex-row">
            {#if addStep === ADD_STEP_SERIES && selectedSonarr}
              <Button onclick={goToMatchStep}>Next</Button>
            {/if}
            {#if addStep === ADD_STEP_FALLBACK_AFL && selectedFallbackAfl}
              <Button onclick={goToFallbackSonarrStep}>Next</Button>
            {/if}
            {#if addStep === ADD_STEP_FALLBACK_SONARR}
              <Button onclick={handleAddSelectedSonarrLookup} disabled={sonarrAdding}>
                {#if sonarrAdding}
                  <LoaderCircle size={16} class="animate-spin" />
                {:else}
                  <Plus size={16} />
                {/if}
                {sonarrAdding ? 'Adding to Sonarr...' : 'Add to Sonarr'}
              </Button>
            {/if}
            {#if addStep === ADD_STEP_MATCH && selectedAfl}
              <Button onclick={handlePrimaryMatchAction} disabled={confirming}>
                {#if confirming}
                  <LoaderCircle size={16} class="animate-spin" />
                {:else}
                  {#if isUnsafeMatch(selectedAfl)}
                    <CircleAlert size={16} />
                  {:else}
                    <Check size={16} />
                  {/if}
                {/if}
                {#if confirming}
                  Adding...
                {:else if isUnsafeMatch(selectedAfl)}
                  Review unsafe match
                {:else}
                  Add anime
                {/if}
              </Button>
            {/if}
            {#if addStep === ADD_STEP_UNSAFE_MATCH}
              <Button variant="secondary" onclick={handleKeepSonarrOnly}>Keep Sonarr only</Button>
              <Button onclick={handleConfirmUnsafeOverride} disabled={confirming}>
                {#if confirming}
                  <LoaderCircle size={16} class="animate-spin" />
                {:else}
                  <Check size={16} />
                {/if}
                {confirming ? 'Creating mapping...' : 'Create mapping anyway'}
              </Button>
            {/if}
            {#if addStep === ADD_STEP_POST_ADD}
              {#if addOutcome?.type === 'mapping_created'}
                <Button variant="secondary" onclick={closeAddModal}>Close</Button>
                <Button onclick={() => {
                  const id = addOutcome?.mappingId || (($shows || []).find((show) => show.sonarr_series_id === selectedSonarr?.id)?.id);
                  if (id) openShow(id);
                  else closeAddModal();
                }}>
                  <Check size={16} />
                  View anime
                </Button>
              {:else}
                <Button onclick={closeAddModal}>
                  <Check size={16} />
                  Done
                </Button>
              {/if}
            {/if}
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}
