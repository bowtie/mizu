<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { ActionReturn } from 'svelte/action';
  import { autoAnimateAction as autoAnimate } from '../../lib/autoAnimate.ts';
  import { goto } from '$app/navigation';
  import type { AFLMatch, MappingCreate, SelectOption, Settings, Show, SonarrAddOptions, SonarrAddPayload, SonarrImage, SonarrSeries } from '$lib/types.ts';
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
  import {
    createPosterMonogram,
    POSTER_FAILED,
    POSTER_LOADING,
    resolvePosterElementState,
  } from '../../lib/poster-state.ts';
  import { resolveSonarrImageUrl } from '../../lib/sonarr-images.ts';
  import { Switch } from '@skeletonlabs/skeleton-svelte';
  import Select from '../../components/Select.svelte';
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

  let loading = $state(true);
  let filter = $state('');
  let sortBy = $state('title');

  let allSeries = $state<SonarrSeries[]>([]);
  let addSeriesLibrary = $state<SonarrSeries[]>([]);
  let seriesById = $state<Record<string | number, SonarrSeries>>({});

  let showAddModal = $state(false);
  let addDialog: HTMLDialogElement | null = $state(null);
  let addStep = $state(ADD_STEP_SERIES);
  let sonarrLoading = $state(false);
  let seriesQuery = $state('');
  let selectedSonarr = $state<SonarrSeries | null>(null);
  let suggestedMatches = $state<AFLMatch[]>([]);
  let displayedMatches = $state<AFLMatch[]>([]);
  let matchLoading = $state(false);
  let manualQuery = $state('');
  let manualSearching = $state(false);
  let manualSearchVisible = $state(false);
  let selectedAfl = $state<AFLMatch | null>(null);
  let confirming = $state(false);
  let addTrackFiller = $state(false);
  let addTrackMixed = $state(true);
  let addTrackAnimeCanon = $state(true);
  let addStartMode = $state(START_FROM_CURRENT);
  let fallbackAflQuery = $state('');
  let cachedAflShows = $state<AFLMatch[]>([]);
  let fallbackAflResults = $state<AFLMatch[]>([]);
  let fallbackAflLoading = $state(false);
  let selectedFallbackAfl = $state<AFLMatch | null>(null);
  let fallbackFilterTimeout: ReturnType<typeof setTimeout> | null = $state(null);
  let sonarrLookupQuery = $state('');
  let sonarrLookupResults = $state<SonarrSeries[]>([]);
  let sonarrLookupLoading = $state(false);
  let selectedSonarrLookup = $state<SonarrSeries | null>(null);
  let sonarrAdding = $state(false);
  let sonarrRootFolderOptions = $state<SelectOption[]>([]);
  let sonarrQualityProfileOptions = $state<SelectOption[]>([]);
  let sonarrAddOptionsError = $state('');
  let fallbackQualityProfileId = $state('');
  let fallbackSeasonFolder = $state(true);
  let addOutcome = $state<AddOutcome | null>(null);

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
  const activeWizardSteps = $derived([ADD_STEP_FALLBACK_AFL, ADD_STEP_FALLBACK_SONARR].includes(addStep) ? wizardFallbackSteps : wizardSteps);
  const currentStepIndex = $derived(activeWizardSteps.findIndex((s: WizardStep) => s.key === addStep));
  const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

  const mappedShows = $derived(($shows || []).map((show: Show) => {
    const series = seriesById[show.sonarr_series_id] || null;
    return {
      ...show,
      poster: ($settings?.enable_anilist_artwork && show.anilist_poster_url) ? show.anilist_poster_url : posterUrl(series),
    };
  }));

  const filteredShows = $derived(mappedShows
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
    }));

  const availableSeries = $derived(allSeries
    .filter((series: SonarrSeries) => !new Set(($shows || []).map((show: Show) => show.sonarr_series_id)).has(series.id))
    .sort((a: SonarrSeries, b: SonarrSeries) => a.title.localeCompare(b.title)));

  const mappedSeriesIds = $derived(new Set(($shows || []).map((show: Show) => show.sonarr_series_id)));

  const filteredSeriesLibrary = $derived(addSeriesLibrary
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
    }));

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
    addDialog?.showModal();

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
    addDialog?.close();
  }

  function onDialogClose(): void {
    showAddModal = false;
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
</script>

<svelte:head>
  <title>Mizu - Anime</title>
</svelte:head>

<div class="space-y-6">
  <section class="flex flex-wrap items-end gap-x-4 gap-y-3">
    <div class="space-y-1.5">
      <h1 class="text-2xl font-bold tracking-tight text-surface-50">Anime</h1>
      <p class="text-sm text-surface-400">
        {#if !loading}
          <span class="tabular-nums text-surface-50">{($shows || []).length}</span> tracked ·
        {/if}
        Tracked series, sync status, and filler mapping.
      </p>
    </div>
    <div class="ml-auto">
      <button class="btn btn-base preset-filled-primary-500" onclick={openAddModal}>
        <Plus size={16} />
        Add anime
      </button>
    </div>
  </section>

  <!-- Search + sort strip -->
  <section class="flex flex-col gap-2 sm:flex-row sm:items-center">
    <div class="relative min-w-0 flex-1">
      <span class="pointer-events-none absolute inset-y-0 left-0 z-10 flex items-center pl-3.5 text-surface-400">
        <Search size={16} />
      </span>
      <input
        type="text"
        placeholder="Search anime by title or slug"
        bind:value={filter}
        class="input field-base h-11 w-full rounded-lg pl-10 text-sm shadow-none"
        aria-label="Search anime"
      />
    </div>
    <div class="sm:w-52">
      <Select options={sortOptions} bind:value={sortBy} placeholder="Sort by" />
    </div>
  </section>

  {#if loading}
    <div class="card preset-tonal-surface flex min-h-56 flex-col items-center justify-center gap-3 text-center">
      <LoaderCircle size={26} class="animate-spin text-primary-500" />
      <div class="space-y-1">
        <h2 class="text-sm font-semibold text-surface-50">Loading anime</h2>
        <p class="text-sm text-surface-400">Loading mappings and Sonarr state.</p>
      </div>
    </div>
  {:else if filteredShows.length === 0 && ($shows || []).length === 0}
    <div class="card preset-tonal-surface flex min-h-72 flex-col items-center justify-center gap-4 px-6 text-center">
      <span class="card flex size-10 items-center justify-center text-surface-400">
        <Waves size={18} />
      </span>
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-surface-50">No anime synced yet</h2>
        <p class="max-w-md text-sm leading-6 text-surface-400">Add your first Sonarr anime mapping to start monitoring filler decisions in Mizu.</p>
      </div>
      <button class="btn btn-base preset-filled-primary-500" onclick={openAddModal}>
        <Plus size={16} />
        Add anime
      </button>
    </div>
  {:else if filteredShows.length === 0}
    <div class="card preset-tonal-surface flex min-h-56 flex-col items-center justify-center gap-4 px-6 text-center">
      <span class="card flex size-10 items-center justify-center text-surface-400">
        <CircleAlert size={18} />
      </span>
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-surface-50">No matching anime</h2>
        <p class="text-sm text-surface-400">Try another title, slug, or keyword.</p>
      </div>
    </div>
  {:else}
    <div class="rule-label"><span>Tracked</span></div>
    <section class="overflow-hidden rounded-lg" use:autoAnimate>
      {#each filteredShows as show}
        {@const series = seriesById[show.sonarr_series_id]}
        {@const episodeCount = series?.statistics?.episodeCount}
        {@const banner = bannerUrl(show, series)}
        <a
          href={`/shows/${show.id}`}
          class="group relative flex h-16 items-center gap-3.5 overflow-hidden border-b border-surface-800/60 px-4 transition-colors last:border-b-0 hover:bg-surface-800/30 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary-500 sm:gap-4 sm:px-5"
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
                class="h-full w-full object-cover opacity-[0.35] transition-all duration-500 ease-out group-hover:scale-105 group-hover:opacity-[0.45]"
              />
              <div class="absolute inset-0 bg-gradient-to-r from-surface-950 via-surface-950/70 to-surface-950/20"></div>
            </div>
          {/if}

          <div class="relative z-10 min-w-0 flex-1">
            <h2 class="line-clamp-1 text-sm font-semibold text-surface-50">{show.sonarr_title}</h2>
            <p class="mt-0.5 text-[11px] tabular-nums text-surface-300">{markerChip(show)}</p>
          </div>

          <span class={`badge badge-sm text-[10px] uppercase tracking-[0.08em] ${show.enabled ? 'preset-filled-success-500' : 'preset-tonal-surface'}`}>
            {show.enabled ? 'Syncing' : 'Paused'}
          </span>
        </a>
      {/each}
    </section>
  {/if}
</div>

<!-- Add anime modal (native <dialog> + Skeleton dialog classes) -->
<dialog bind:this={addDialog} onclose={onDialogClose} class="dialog animate-dialog preset-filled-surface-100-900 max-h-[calc(100vh-1.5rem)] w-full max-w-3xl sm:max-h-[calc(100vh-3rem)]" aria-labelledby="show-add-modal-title">
  <!-- Header -->
  <header class="space-y-3 border-b border-surface-700 px-5 pb-4 pt-5 sm:px-7">
    <div class="flex items-center gap-3">
      <h2 id="show-add-modal-title" class="text-xl font-semibold tracking-tight text-surface-50">Add anime</h2>
      <button
        type="button"
        class="btn-icon btn-icon-sm preset-tonal ml-auto shrink-0"
        onclick={closeAddModal}
        aria-label="Close add anime modal"
      >
        <X size={15} />
      </button>
    </div>

    <!-- Step progress -->
    <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[10px] uppercase tracking-[0.1em]" aria-hidden="true">
      {#each activeWizardSteps as step, i}
        {@const isCurrent = step.key === addStep}
        {@const isPast = currentStepIndex > i}
        {#if i > 0}
          <span class="h-px w-4 bg-surface-600"></span>
        {/if}
        <span class="flex items-center gap-1.5 {isCurrent ? 'text-surface-50' : isPast ? 'text-surface-300' : 'text-surface-500'}">
          {#if isPast}
            <Check size={10} class="text-primary-500" />
          {/if}
          {step.label}
        </span>
      {/each}
    </div>

    <p class="text-sm leading-6 text-surface-400">
      {#if addStep === ADD_STEP_SERIES}
        Search your actionable Sonarr anime library first. If the anime is missing, switch to cached AFL search and add it into Sonarr from here.
      {:else if addStep === ADD_STEP_FALLBACK_AFL}
        Search cached AnimeFillerList titles, then use the chosen title to look up the right Sonarr series before adding it.
      {:else if addStep === ADD_STEP_FALLBACK_SONARR}
        Confirm the Sonarr series Mizu should add for <span class="font-medium text-surface-300">{selectedFallbackAfl?.title}</span>.
      {:else if addStep === ADD_STEP_POST_ADD}
        {#if addOutcome?.type === 'mapping_created'}
          <span class="font-medium text-surface-300">{addOutcome?.title}</span> is now tracked by Mizu.
        {:else}
          <span class="font-medium text-surface-300">{addOutcome?.title}</span> is in Sonarr now, but still needs a safe AFL mapping before Mizu can track it.
        {/if}
      {:else if addStep === ADD_STEP_UNSAFE_MATCH}
        Mizu found a possible cached AFL entry for <span class="font-medium text-surface-300">{selectedSonarr?.title}</span>, but it needs your confirmation before creating the mapping.
      {:else}
        Match <span class="font-medium text-surface-300">{selectedSonarr?.title}</span> with the correct AnimeFillerList entry.
      {/if}
    </p>
  </header>

  <!-- Scrollable body -->
  <article class="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-5 sm:px-7 sm:py-6">
    {#if addStep === ADD_STEP_SERIES}
      {#if sonarrLoading}
        <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
          <LoaderCircle size={28} class="animate-spin text-primary-500" />
          <p class="text-sm text-surface-400">Loading available Sonarr anime.</p>
        </div>
      {:else if allSeries.length === 0}
        <div class="card p-6 text-center">
          <h3 class="text-sm font-semibold text-surface-50">No anime found in Sonarr</h3>
          <p class="mt-2 text-sm leading-6 text-surface-400">Sonarr returned no anime series for this library.</p>
        </div>
      {:else}
        <div class="space-y-4">
          <div class="space-y-2">
            <label for="show-add-series-search" class="label label-text">Search</label>
            <div class="relative">
              <div class="pointer-events-none absolute inset-y-0 left-3 z-10 flex items-center text-surface-400">
                <Search size={16} />
              </div>
              <input
                id="show-add-series-search"
                type="text"
                placeholder="Search Sonarr anime"
                class="input field-base pl-10"
                bind:value={seriesQuery}
              />
            </div>
          </div>

          {#if filteredSeriesLibrary.length === 0}
            <div class="card space-y-4 p-6 text-center">
              <h3 class="text-sm font-semibold text-surface-50">No anime match that search</h3>
              <p class="mt-2 text-sm leading-6 text-surface-400">Try a different title, year, or jump into the cached AFL list and add it from there.</p>
              <div class="flex justify-center">
                <button class="btn btn-base preset-tonal" onclick={() => goToFallbackAflStep(seriesQuery.trim())}>
                  <Search size={16} />
                  Not in Sonarr? Search AFL and add to Sonarr
                </button>
              </div>
            </div>
          {:else}
            <div class="card h-[23rem] overflow-y-auto">
              <div class="divide-y divide-surface-800">
                {#each filteredSeriesLibrary as series}
                  {@const seriesPoster = posterUrl(series)}
                  {@const seriesMapped = mappedSeriesIds.has(series.id)}
                  {@const manualPending = !!series.manualMatchPending}
                  {@const isSelected = selectedSonarr?.id === series.id}
                  <button
                    type="button"
                    class="group flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary-500/10' : seriesMapped ? 'opacity-60 hover:bg-surface-800' : 'hover:bg-surface-800'}"
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
                      class="poster-frame poster-thumb relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-[5px] border border-surface-700 bg-surface-800 sm:w-11"
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
                      <div class="poster-fallback bg-surface-800 text-sm font-semibold tracking-tight text-surface-400">
                        {createPosterMonogram(series.title)}
                      </div>
                    </div>

                    <div class="min-w-0 flex-1 space-y-0.5">
                      <div class="line-clamp-1 text-sm font-medium text-surface-50">{series.title}</div>
                      <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px] tabular-nums text-surface-500">
                        <span>{series.year || 'Anime'}</span>
                        {#if series.statistics?.episodeCount}
                          <span aria-hidden="true">·</span>
                          <span>{series.statistics.episodeCount} eps</span>
                        {/if}
                        {#if manualPending}
                          <span aria-hidden="true">·</span>
                          <span class="text-warning-500">Needs match</span>
                        {:else if seriesMapped}
                          <span aria-hidden="true">·</span>
                          <span>Already added</span>
                        {/if}
                      </div>
                    </div>

                    {#if seriesMapped}
                      <span class="shrink-0 text-[10px] uppercase tracking-[0.1em] text-surface-400">Open</span>
                    {:else if manualPending}
                      <span class="shrink-0 text-[10px] uppercase tracking-[0.1em] text-warning-500">Match</span>
                    {:else if isSelected}
                      <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-500 text-primary-contrast-500">
                        <Check size={13} />
                      </div>
                    {:else}
                      <div class="h-6 w-6 shrink-0 rounded-full border border-surface-600 group-hover:border-primary-500/40"></div>
                    {/if}
                  </button>
                {/each}
              </div>
            </div>
          {/if}

        </div>
      {/if}
    {:else if addStep === ADD_STEP_FALLBACK_AFL}
      <div class="space-y-4">
        <div class="space-y-2">
          <label for="fallback-afl-search" class="label label-text">Cached AFL title</label>
          <input
            id="fallback-afl-search"
            type="text"
            placeholder="Filter cached AnimeFillerList titles"
            class="input field-base"
            bind:value={fallbackAflQuery}
            oninput={scheduleFallbackAflFilter}
          />
        </div>

        {#if fallbackAflLoading}
          <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
            <LoaderCircle size={28} class="animate-spin text-primary-500" />
            <p class="text-sm text-surface-400">Loading cached AnimeFillerList titles.</p>
          </div>
        {:else if fallbackAflResults.length === 0}
          <div class="card p-6 text-center">
            <h3 class="text-sm font-semibold text-surface-50">No cached AFL titles found</h3>
            <p class="mt-2 text-sm leading-6 text-surface-400">Try another filter or refresh the AFL cache in Settings if this anime is new.</p>
          </div>
        {:else}
          <div class="card h-[22rem] overflow-y-auto">
            <div class="divide-y divide-surface-800">
              {#each fallbackAflResults as result}
                {@const isSelected = selectedFallbackAfl?.slug === result.slug}
                <button
                  type="button"
                  class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary-500/10' : 'hover:bg-surface-800'}"
                  onclick={() => (selectedFallbackAfl = result)}
                >
                  <div class="min-w-0">
                    <div class="truncate text-sm font-medium text-surface-50">{result.title}</div>
                  </div>
                  {#if isSelected}
                    <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-500 text-primary-contrast-500">
                      <Check size={13} />
                    </div>
                  {:else}
                    <div class="h-6 w-6 shrink-0 rounded-full border border-surface-600"></div>
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {:else if addStep === ADD_STEP_FALLBACK_SONARR}
      <div class="space-y-4">
        <div class="space-y-2">
          <label for="fallback-sonarr-search" class="label label-text">Sonarr lookup</label>
          <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
            <input
              id="fallback-sonarr-search"
              type="text"
              placeholder="Search Sonarr for the selected AFL anime"
              class="input field-base"
              bind:value={sonarrLookupQuery}
              onkeydown={(event) => event.key === 'Enter' && loadSonarrLookupResults()}
            />
            <button class="btn btn-base preset-tonal" onclick={loadSonarrLookupResults} disabled={sonarrLookupLoading || !sonarrLookupQuery.trim()}>
              {#if sonarrLookupLoading}
                <LoaderCircle size={16} class="animate-spin" />
              {:else}
                <Search size={16} />
              {/if}
              Search
            </button>
          </div>
        </div>

        {#if sonarrLookupLoading}
          <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
            <LoaderCircle size={28} class="animate-spin text-primary-500" />
            <p class="text-sm text-surface-400">Looking up Sonarr candidates.</p>
          </div>
        {:else if sonarrLookupResults.length === 0}
          <div class="card p-6 text-center">
            <h3 class="text-sm font-semibold text-surface-50">No Sonarr results found</h3>
            <p class="mt-2 text-sm leading-6 text-surface-400">Try another title variation for {selectedFallbackAfl?.title} and search again.</p>
          </div>
        {:else}
          <div class="card h-[18rem] overflow-y-auto">
            <div class="divide-y divide-surface-800">
              {#each sonarrLookupResults as result}
                {@const lookupPoster = posterUrl(result)}
                {@const isSelected = selectedSonarrLookup?.tvdbId === result.tvdbId && selectedSonarrLookup?.title === result.title}
                <button
                  type="button"
                  class="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary-500/10' : 'hover:bg-surface-800'}"
                  onclick={() => (selectedSonarrLookup = result)}
                >
                  <div
                    class="poster-frame poster-thumb relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-[5px] border border-surface-700 bg-surface-800 sm:w-11"
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
                    <div class="poster-fallback bg-surface-800 text-sm font-semibold tracking-tight text-surface-400">
                      {createPosterMonogram(result.title)}
                    </div>
                  </div>

                  <div class="min-w-0 flex-1 space-y-0.5">
                    <div class="truncate text-sm font-medium text-surface-50">{result.title}</div>
                    <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px] text-surface-500">
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
                    <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-500 text-primary-contrast-500">
                      <Check size={13} />
                    </div>
                  {:else}
                    <div class="h-6 w-6 shrink-0 rounded-full border border-surface-600"></div>
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <div class="card grid gap-3 p-4 md:grid-cols-[minmax(0,1fr)_18rem] md:items-start">
          <div class="space-y-2">
            <label for="fallback-quality-profile" class="label label-text">Quality profile</label>
            <div class="max-w-xs">
              <Select options={sonarrQualityProfileOptions} bind:value={fallbackQualityProfileId} placeholder="Choose a quality profile" />
            </div>
          </div>

          <label class="card flex items-center justify-between gap-4 bg-surface-950 px-4 py-3">
            <div class="text-sm font-medium text-surface-50">Season folder</div>
            <Switch checked={fallbackSeasonFolder} onCheckedChange={(d) => (fallbackSeasonFolder = d.checked)}>
              <Switch.Control>
                <Switch.Thumb />
              </Switch.Control>
              <Switch.HiddenInput />
            </Switch>
          </label>

          {#if sonarrAddOptionsError}
            <div class="card preset-filled-warning-500 px-4 py-3 text-sm">
              Unable to load Sonarr quality profiles right now: {sonarrAddOptionsError}
            </div>
          {/if}
        </div>
      </div>
    {:else if addStep === ADD_STEP_UNSAFE_MATCH}
      <div class="card space-y-4 border border-warning-500/30 bg-warning-500/10 p-5">
        <div class="flex items-start gap-3">
          <span class="card mt-0.5 flex size-8 shrink-0 items-center justify-center border border-warning-500/30 text-warning-500">
            <CircleAlert size={15} />
          </span>
          <div class="space-y-2">
            <h3 class="text-sm font-semibold text-surface-50">This cached AFL match needs confirmation</h3>
            <p class="text-sm leading-6 text-warning-500/90">
              Mizu could not safely verify that <span class="font-medium text-surface-50">{matchTitle(selectedAfl)}</span> is the right AnimeFillerList entry for
              <span class="font-medium text-surface-50"> {selectedSonarr?.title}</span>. You can still create the mapping manually, or keep the series in Sonarr only and match it later.
            </p>
          </div>
        </div>

        <div class="card bg-surface-950 p-4">
          <div class="text-sm font-semibold text-surface-50">{matchTitle(selectedAfl)}</div>
          <div class="mt-1 text-xs text-surface-400">{matchSlug(selectedAfl)}</div>
        </div>
      </div>
    {:else if addStep === ADD_STEP_POST_ADD}
      <div class="card space-y-4 p-5">
        <div class="flex items-start gap-3">
          <span class="card mt-0.5 flex size-8 shrink-0 items-center justify-center border border-success-500/30 text-success-500">
            <CircleCheckBig size={15} />
          </span>
          <div class="space-y-2">
            {#if addOutcome?.type === 'mapping_created'}
              <h3 class="text-sm font-semibold text-surface-50">Mapping created</h3>
              <p class="text-sm leading-6 text-surface-400">
                <span class="font-medium text-surface-50">{addOutcome?.title}</span> is now tracked by Mizu. You can configure its tracking rules and episode monitoring on the detail page.
              </p>
            {:else}
              <h3 class="text-sm font-semibold text-surface-50">Added to Sonarr</h3>
              <p class="text-sm leading-6 text-surface-400">
                {addOutcome?.title || selectedSonarr?.title} is in Sonarr now. Mizu did not create a mapping because it could not confirm a safe AFL match, so this anime will stay visible in the add modal as <span class="font-medium text-surface-50">Needs match</span>.
              </p>
            {/if}
          </div>
        </div>
      </div>
    {:else}
      {#if manualSearchVisible}
        <div class="card space-y-3 p-4">
          <div class="space-y-1">
            <label for="afl-search" class="label label-text">Manual search</label>
            <p class="text-sm text-surface-400">Automatic matching was not strong enough, so you can search the cached AnimeFillerList titles directly.</p>
          </div>
          <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
            <input
              id="afl-search"
              type="text"
              placeholder="Search cached AnimeFillerList titles"
              class="input field-base"
              bind:value={manualQuery}
              onkeydown={(event) => event.key === 'Enter' && handleManualSearch()}
            />
            <button class="btn btn-base preset-tonal" onclick={handleManualSearch} disabled={manualSearching}>
              {#if manualSearching}
                <LoaderCircle size={16} class="animate-spin" />
              {:else}
                <Search size={16} />
              {/if}
              Search
            </button>
          </div>
        </div>
      {/if}

      {#if matchLoading}
        <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
          <LoaderCircle size={28} class="animate-spin text-primary-500" />
          <p class="text-sm text-surface-400">Looking for strong cached AnimeFillerList matches.</p>
        </div>
      {:else if displayedMatches.length === 0}
        <div class="card p-6 text-center">
          <h3 class="text-sm font-semibold text-surface-50">No matches found</h3>
          <p class="mt-2 text-sm leading-6 text-surface-400">Refresh the AnimeFillerList cache in Settings if this anime is new, then try again.</p>
        </div>
      {:else if selectedAfl && isCompactMatchSelection(selectedAfl)}
        <div class="card space-y-4 p-5">
          <div class="space-y-2">
            <div class="text-sm font-semibold text-surface-50">{matchTitle(selectedAfl)}</div>
            <div class="flex flex-wrap gap-2">
              <span class="badge badge-sm text-[10px] uppercase tracking-[0.1em] preset-filled-primary-500">{Math.round((selectedAfl.confidence || 0) * 100)}% confidence</span>
              <span class="badge badge-sm text-[10px] uppercase tracking-[0.1em] preset-tonal-surface">{selectedAfl.method}</span>
            </div>
            <p class="text-sm leading-6 text-surface-400">Looks like an exact cached AFL match, so this step is just a quick confirmation before Mizu creates the mapping.</p>
          </div>
        </div>
      {:else}
        <div class="card h-[18rem] overflow-y-auto">
          <div class="divide-y divide-surface-800">
            {#each displayedMatches as result}
              {@const isSelected = selectedAfl && matchSlug(selectedAfl) === matchSlug(result)}
              <button
                type="button"
                class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors {isSelected ? 'bg-primary-500/10' : 'hover:bg-surface-800'}"
                onclick={() => (selectedAfl = result)}
              >
                <div class="min-w-0 space-y-1">
                  <div class="truncate text-sm font-medium text-surface-50">{matchTitle(result)}</div>
                  <div class="truncate text-xs text-surface-400">{matchSlug(result)}</div>
                  <div class="flex flex-wrap gap-2">
                    {#if result.confidence != null}
                      <span class="badge badge-sm text-[10px] uppercase tracking-[0.1em] preset-filled-primary-500">{Math.round(result.confidence * 100)}% confidence</span>
                      <span class="badge badge-sm text-[10px] uppercase tracking-[0.1em] preset-tonal-surface">{result.method}</span>
                    {:else}
                      <span class="badge badge-sm text-[10px] uppercase tracking-[0.1em] preset-filled-warning-500">Manual review</span>
                    {/if}
                  </div>
                </div>
                {#if isSelected}
                  <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-500 text-primary-contrast-500">
                    <Check size={13} />
                  </div>
                {:else}
                  <div class="h-6 w-6 shrink-0 rounded-full border border-surface-600"></div>
                {/if}
              </button>
            {/each}
          </div>
        </div>
      {/if}

      {#if selectedAfl}
        <div class="card space-y-4 p-4">
          <div class="space-y-1">
            <h3 class="text-sm font-semibold text-surface-50">Default tracking rules</h3>
            <p class="text-sm text-surface-400">Set the initial monitoring rules for this mapping before it is created.</p>
          </div>

          <div class="grid gap-3 md:grid-cols-3">
            <div class="card flex items-start justify-between gap-3 bg-surface-950 p-4">
              <div class="space-y-1">
                <div class="text-sm font-medium text-surface-50">Anime canon</div>
                <span class="inline-block text-[10px] uppercase tracking-[0.1em] {addTrackAnimeCanon ? 'text-primary-500' : 'text-surface-500'}">{optionState(addTrackAnimeCanon)}</span>
              </div>
              <Switch checked={addTrackAnimeCanon} onCheckedChange={(d) => (addTrackAnimeCanon = d.checked)}>
                <Switch.Control>
                  <Switch.Thumb />
                </Switch.Control>
                <Switch.HiddenInput />
              </Switch>
            </div>

            <div class="card flex items-start justify-between gap-3 bg-surface-950 p-4">
              <div class="space-y-1">
                <div class="text-sm font-medium text-surface-50">Mixed canon</div>
                <span class="inline-block text-[10px] uppercase tracking-[0.1em] {addTrackMixed ? 'text-warning-500' : 'text-surface-500'}">{optionState(addTrackMixed)}</span>
              </div>
              <Switch checked={addTrackMixed} onCheckedChange={(d) => (addTrackMixed = d.checked)}>
                <Switch.Control>
                  <Switch.Thumb />
                </Switch.Control>
                <Switch.HiddenInput />
              </Switch>
            </div>

            <div class="card flex items-start justify-between gap-3 bg-surface-950 p-4">
              <div class="space-y-1">
                <div class="text-sm font-medium text-surface-50">Filler</div>
                <span class="inline-block text-[10px] uppercase tracking-[0.1em] {addTrackFiller ? 'text-error-500' : 'text-surface-500'}">{optionState(addTrackFiller)}</span>
              </div>
              <Switch checked={addTrackFiller} onCheckedChange={(d) => (addTrackFiller = d.checked)}>
                <Switch.Control>
                  <Switch.Thumb />
                </Switch.Control>
                <Switch.HiddenInput />
              </Switch>
            </div>
          </div>

          <div class="space-y-2">
            <label for="start-mode" class="label label-text">Start tracking from</label>
            <Select options={startModeOptions} bind:value={addStartMode} placeholder="Choose start point" />
            <p class="text-sm text-surface-400">Current progress uses Sonarr's latest monitored episode. Episode 1 starts from the beginning.</p>
          </div>
        </div>
      {/if}
    {/if}
  </article>

  <!-- Footer with actions -->
  <footer class="flex shrink-0 flex-col-reverse gap-3 border-t border-surface-700 px-5 py-4 sm:flex-row sm:justify-between sm:px-7">
    <div class="flex flex-col-reverse gap-3 sm:flex-row">
      {#if addStep !== ADD_STEP_POST_ADD}
        <button class="btn btn-base preset-tonal" onclick={closeAddModal}>Cancel</button>
      {/if}
      {#if addStep === ADD_STEP_FALLBACK_AFL}
        <button class="btn btn-base preset-tonal" onclick={() => returnToSeriesStep()}>Back to Sonarr</button>
      {:else if addStep === ADD_STEP_FALLBACK_SONARR}
        <button class="btn btn-base preset-tonal" onclick={() => (addStep = ADD_STEP_FALLBACK_AFL)}>Back to AFL</button>
      {:else if addStep === ADD_STEP_MATCH}
        <button class="btn btn-base preset-tonal" onclick={() => returnToSeriesStep(selectedSonarr?.title || '')}>Change anime</button>
      {:else if addStep === ADD_STEP_UNSAFE_MATCH}
        <button class="btn btn-base preset-tonal" onclick={() => (addStep = ADD_STEP_MATCH)}>Back to matches</button>
      {/if}
    </div>

    <div class="flex flex-col gap-3 sm:flex-row">
      {#if addStep === ADD_STEP_SERIES && selectedSonarr}
        <button class="btn btn-base preset-filled-primary-500" onclick={goToMatchStep}>Next</button>
      {/if}
      {#if addStep === ADD_STEP_FALLBACK_AFL && selectedFallbackAfl}
        <button class="btn btn-base preset-filled-primary-500" onclick={goToFallbackSonarrStep}>Next</button>
      {/if}
      {#if addStep === ADD_STEP_FALLBACK_SONARR}
        <button class="btn btn-base preset-filled-primary-500" onclick={handleAddSelectedSonarrLookup} disabled={sonarrAdding}>
          {#if sonarrAdding}
            <LoaderCircle size={16} class="animate-spin" />
          {:else}
            <Plus size={16} />
          {/if}
          {sonarrAdding ? 'Adding to Sonarr...' : 'Add to Sonarr'}
        </button>
      {/if}
      {#if addStep === ADD_STEP_MATCH && selectedAfl}
        <button class="btn btn-base preset-filled-primary-500" onclick={handlePrimaryMatchAction} disabled={confirming}>
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
        </button>
      {/if}
      {#if addStep === ADD_STEP_UNSAFE_MATCH}
        <button class="btn btn-base preset-tonal" onclick={handleKeepSonarrOnly}>Keep Sonarr only</button>
        <button class="btn btn-base preset-filled-primary-500" onclick={handleConfirmUnsafeOverride} disabled={confirming}>
          {#if confirming}
            <LoaderCircle size={16} class="animate-spin" />
          {:else}
            <Check size={16} />
          {/if}
          {confirming ? 'Creating mapping...' : 'Create mapping anyway'}
        </button>
      {/if}
      {#if addStep === ADD_STEP_POST_ADD}
        {#if addOutcome?.type === 'mapping_created'}
          <button class="btn btn-base preset-tonal" onclick={closeAddModal}>Close</button>
          <button class="btn btn-base preset-filled-primary-500" onclick={() => {
            const id = addOutcome?.mappingId || (($shows || []).find((show) => show.sonarr_series_id === selectedSonarr?.id)?.id);
            if (id) openShow(id);
            else closeAddModal();
          }}>
            <Check size={16} />
            View anime
          </button>
        {:else}
          <button class="btn btn-base preset-filled-primary-500" onclick={closeAddModal}>
            <Check size={16} />
            Done
          </button>
        {/if}
      {/if}
    </div>
  </footer>
</dialog>
