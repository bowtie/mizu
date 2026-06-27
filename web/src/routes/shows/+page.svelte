<script>
  import { onDestroy, onMount } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { goto } from '$app/navigation';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Checkbox } from '$lib/components/ui/checkbox/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import Box from '$lib/components/Box.svelte';
  import { shows, settings, addNotification } from '../../lib/stores.js';
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
  } from '../../lib/api.js';
  import { episodeTypeBadge, syncStateBadge } from '../../lib/fox.js';
  import {
    createPosterMonogram,
    POSTER_FAILED,
    POSTER_LOADING,
    resolvePosterElementState,
  } from '../../lib/poster-state.js';
  import { resolveSonarrImageUrl } from '../../lib/sonarr-images.js';
  import { Check, CircleAlert, CircleCheckBig, LoaderCircle, Plus, Search, Waves, X } from 'lucide-svelte';

  const START_FROM_BEGINNING = 'from_beginning';
  const START_FROM_CURRENT = 'from_current_progress';
  const ADD_STEP_SERIES = 'series';
  const ADD_STEP_FALLBACK_AFL = 'fallback_afl';
  const ADD_STEP_FALLBACK_SONARR = 'fallback_sonarr';
  const ADD_STEP_MATCH = 'match';
  const ADD_STEP_UNSAFE_MATCH = 'unsafe_match';
  const ADD_STEP_POST_ADD = 'post_add';

  let loading = true;
  let filter = '';
  let sortBy = 'title';

  let allSeries = [];
  let addSeriesLibrary = [];
  let seriesById = {};

  let showAddModal = false;
  let addStep = ADD_STEP_SERIES;
  let sonarrLoading = false;
  let seriesQuery = '';
  let selectedSonarr = null;
  let suggestedMatches = [];
  let displayedMatches = [];
  let matchLoading = false;
  let manualQuery = '';
  let manualSearching = false;
  let manualSearchVisible = false;
  let selectedAfl = null;
  let confirming = false;
  let addTrackFiller = false;
  let addTrackMixed = true;
  let addTrackAnimeCanon = true;
  let addStartMode = START_FROM_CURRENT;
  let fallbackAflQuery = '';
  let cachedAflShows = [];
  let fallbackAflResults = [];
  let fallbackAflLoading = false;
  let selectedFallbackAfl = null;
  let fallbackFilterTimeout = null;
  let sonarrLookupQuery = '';
  let sonarrLookupResults = [];
  let sonarrLookupLoading = false;
  let selectedSonarrLookup = null;
  let sonarrAdding = false;
  let sonarrRootFolderOptions = [];
  let sonarrQualityProfileOptions = [];
  let sonarrAddOptionsError = '';
  let fallbackQualityProfileId = '';
  let fallbackSeasonFolder = true;
  let addOutcome = null;

  const startModeOptions = [
    { value: START_FROM_CURRENT, label: 'Current progress' },
    { value: START_FROM_BEGINNING, label: 'Episode 1' },
  ];
  const sortOptions = [
    { value: 'title', label: 'Title (A-Z)' },
    { value: 'title_desc', label: 'Title (Z-A)' },
    { value: 'watermark', label: 'Latest progress' },
  ];
  const wizardSteps = [
    { key: ADD_STEP_SERIES, label: 'Select' },
    { key: ADD_STEP_MATCH, label: 'Match' },
    { key: ADD_STEP_UNSAFE_MATCH, label: 'Confirm' },
    { key: ADD_STEP_POST_ADD, label: 'Done' },
  ];
  const wizardFallbackSteps = [
    { key: ADD_STEP_FALLBACK_AFL, label: 'AFL Search' },
    { key: ADD_STEP_FALLBACK_SONARR, label: 'Add to Sonarr' },
    { key: ADD_STEP_MATCH, label: 'Match' },
  ];
  $: activeWizardSteps = [ADD_STEP_FALLBACK_AFL, ADD_STEP_FALLBACK_SONARR].includes(addStep) ? wizardFallbackSteps : wizardSteps;
  $: currentStepIndex = activeWizardSteps.findIndex((s) => s.key === addStep);
  const modalSelectContentProps = { class: 'z-[70] p-1' };
  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  $: mappedShows = ($shows || []).map((show) => {
    const series = seriesById[show.sonarr_series_id] || null;
    return {
      ...show,
      poster: posterUrl(series),
    };
  });

  $: filteredShows = mappedShows
    .filter((show) => {
      if (!filter) return true;
      const query = filter.toLowerCase();
      return (show.sonarr_title || '').toLowerCase().includes(query) || (show.afl_title || show.afl_slug || '').toLowerCase().includes(query);
    })
    .sort((a, b) => {
      if (sortBy === 'title') return (a.sonarr_title || '').localeCompare(b.sonarr_title || '');
      if (sortBy === 'title_desc') return (b.sonarr_title || '').localeCompare(a.sonarr_title || '');
      if (sortBy === 'watermark') return (b.sync_watermark || 0) - (a.sync_watermark || 0);
      return 0;
    });

  $: availableSeries = allSeries
    .filter((series) => !new Set(($shows || []).map((show) => show.sonarr_series_id)).has(series.id))
    .sort((a, b) => a.title.localeCompare(b.title));

  $: mappedSeriesIds = new Set(($shows || []).map((show) => show.sonarr_series_id));

  $: filteredSeriesLibrary = addSeriesLibrary
    .filter((series) => {
    const query = seriesQuery.trim().toLowerCase();
    if (!query) return true;
    return (
      (series.title || '').toLowerCase().includes(query) ||
      String(series.year || '').includes(query)
    );
    })
    .sort((a, b) => {
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

  async function loadLanding() {
    loading = true;
    try {
      const [showsRes, seriesRes, settingsRes] = await Promise.allSettled([getShows(), getSonarrSeries(), getSettings()]);
      const nextShows = showsRes.status === 'fulfilled' ? (showsRes.value || []) : [];
      shows.set(nextShows);
      allSeries = seriesRes.status === 'fulfilled' ? (seriesRes.value || []) : [];
      seriesById = Object.fromEntries(allSeries.map((series) => [series.id, series]));
      if (settingsRes.status === 'fulfilled') {
        settings.set(settingsRes.value || {});
      }
    } catch (err) {
      addNotification('Failed to load Mizu anime: ' + err.message, 'error');
    }
    loading = false;
  }

  function posterUrl(series) {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((image) => image.coverType === 'poster') || series.images.find((image) => image.coverType === 'fanart') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, $settings?.sonarr_url || '');
  }

  function markerChip(show) {
    return show.sync_watermark > 0 ? `#${show.sync_watermark}` : '#0';
  }

  function setPosterFrameState(node, state) {
    node.dataset.posterState = state;
  }

  function observePosterFrame(node, show) {
    let currentShow = show;
    let currentImage = null;

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
      update(nextShow) {
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

  function openShow(showId) {
    goto(`/shows/${showId}`);
  }

  async function refreshSettings() {
    try {
      const current = await getSettings();
      settings.set(current || {});
    } catch (err) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
  }

  function resetAddRuleDefaults(current = $settings || {}) {
    addTrackFiller = actionToTracked(current.default_filler_action, false);
    addTrackMixed = actionToTracked(current.default_mixed_canon_action, true);
    addTrackAnimeCanon = actionToTracked(current.default_anime_canon_action, true);
    addStartMode = START_FROM_CURRENT;
  }

  async function loadSonarrAddOptions() {
    sonarrAddOptionsError = '';
    try {
      const data = await getSonarrAddOptions();
      sonarrRootFolderOptions = (data.root_folders || []).map((folder) => ({
        value: folder.path,
        label: folder.path,
      }));
      sonarrQualityProfileOptions = (data.quality_profiles || []).map((profile) => ({
        value: String(profile.id),
        label: profile.name,
      }));
      if (!fallbackQualityProfileId) {
        fallbackQualityProfileId = preferredAnimeQualityProfileId();
      }
    } catch (err) {
      sonarrAddOptionsError = err.message;
    }
  }

  async function refreshAddSeriesLibrary() {
    addSeriesLibrary = (await getSonarrSeries(true)) || [];
  }

  function resetFallbackFlow() {
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

  async function openAddModal() {
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
    resetAddRuleDefaults($settings || {});

    sonarrLoading = true;
    try {
      const [fullSeries, modalSeries] = await Promise.all([
        allSeries.length ? Promise.resolve(allSeries) : getSonarrSeries(),
        getSonarrSeries(true),
      ]);
      allSeries = fullSeries || [];
      addSeriesLibrary = modalSeries || [];
      seriesById = Object.fromEntries(allSeries.map((series) => [series.id, series]));
    } catch (err) {
      addNotification('Failed to load Sonarr series: ' + err.message, 'error');
    }
    sonarrLoading = false;
  }

  function closeAddModal() {
    showAddModal = false;
  }

  function handleAddModalBackdropClick(event) {
    if (event.target === event.currentTarget) {
      closeAddModal();
    }
  }

  function handleWindowKeydown(event) {
    if (showAddModal && event.key === 'Escape') {
      closeAddModal();
    }
  }

  function openExistingMapping(seriesId) {
    const mapped = ($shows || []).find((show) => show.sonarr_series_id === seriesId);
    if (!mapped) return;
    closeAddModal();
    openShow(mapped.id);
  }

  async function goToMatchStep() {
    if (!selectedSonarr) return;
    addStep = ADD_STEP_MATCH;
    manualQuery = selectedSonarr.title || '';
    await loadSuggestedMatches();
  }

  async function loadSuggestedMatches() {
    if (!selectedSonarr) return;
    matchLoading = true;
    selectedAfl = null;
    try {
      suggestedMatches = await suggestAFL(selectedSonarr.id);
      displayedMatches = suggestedMatches;
      manualSearchVisible = suggestedMatches.length === 0 || !!selectedSonarr.manualMatchPending;
      const bestMatch = suggestedMatches[0];
      if (bestMatch && isCompactMatchSelection(bestMatch)) {
        selectedAfl = bestMatch;
      }
      if (manualSearchVisible && selectedSonarr.manualMatchPending && manualQuery.trim()) {
        displayedMatches = await searchAFL(manualQuery.trim(), selectedSonarr.id);
      }
    } catch (err) {
      suggestedMatches = [];
      displayedMatches = [];
      manualSearchVisible = true;
      addNotification('Failed to load AFL suggestions: ' + err.message, 'error');
    }
    matchLoading = false;
  }

  async function handleManualSearch() {
    if (!selectedSonarr || !manualSearchVisible) return;
    manualSearching = true;
    selectedAfl = null;
    try {
      displayedMatches = await searchAFL(manualQuery.trim(), selectedSonarr.id);
    } catch (err) {
      displayedMatches = [];
      addNotification('AFL search failed: ' + err.message, 'error');
    }
    manualSearching = false;
  }

  async function handleConfirmMatch() {
    if (!selectedSonarr || !selectedAfl) return;
    confirming = true;
    try {
      const created = await confirmMatch({
        sonarr_series_id: selectedSonarr.id,
        sonarr_title: selectedSonarr.title,
        afl_slug: selectedAfl.afl_show?.slug || selectedAfl.slug,
        afl_title: selectedAfl.afl_show?.title || selectedAfl.title,
        filler_action: trackedToAction(addTrackFiller),
        mixed_action: trackedToAction(addTrackMixed),
        anime_canon_action: trackedToAction(addTrackAnimeCanon),
        start_mode: addStartMode,
      });
      addNotification(`Synced "${selectedSonarr.title}"`, 'success');
      showAddModal = false;
      await loadLanding();
      if (created?.id) {
        openShow(created.id);
        return;
      }
      const mapped = ($shows || []).find((show) => show.sonarr_series_id === selectedSonarr.id);
      if (mapped) openShow(mapped.id);
    } catch (err) {
      addNotification('Failed to sync anime: ' + err.message, 'error');
    } finally {
      confirming = false;
    }
  }

  function isUnsafeMatch(match) {
    return !!match && match.confidence == null;
  }

  function shouldAutoCreateMappingAfterSonarrAdd(match) {
    return !!match && !isUnsafeMatch(match) && match.method === 'exact' && (match.confidence ?? 0) >= 1;
  }

  function isCompactMatchSelection(match) {
    if (!match || isUnsafeMatch(match)) return false;
    return match.method === 'exact' || match.method === 'normalized' || (match.confidence ?? 0) >= 0.95;
  }

  function preferredAnimeRootFolder() {
    const counts = new Map();
    let bestPath = '';
    let bestCount = 0;

    for (const series of allSeries) {
      const path = (series?.rootFolderPath || '').trim();
      if (!path) continue;
      const nextCount = (counts.get(path) || 0) + 1;
      counts.set(path, nextCount);
      if (nextCount > bestCount) {
        bestCount = nextCount;
        bestPath = path;
      }
    }

    return bestPath || sonarrRootFolderOptions[0]?.value || '';
  }

  function preferredAnimeQualityProfileId() {
    const counts = new Map();
    let bestValue = '';
    let bestCount = 0;

    for (const series of allSeries) {
      const value = Number(series?.qualityProfileId || 0);
      if (!value) continue;
      const key = String(value);
      const nextCount = (counts.get(key) || 0) + 1;
      counts.set(key, nextCount);
      if (nextCount > bestCount) {
        bestCount = nextCount;
        bestValue = key;
      }
    }

    return bestValue || sonarrQualityProfileOptions[0]?.value || '';
  }

  function ensureFallbackAddDefaults() {
    if (!fallbackQualityProfileId) {
      fallbackQualityProfileId = preferredAnimeQualityProfileId();
    }
    fallbackSeasonFolder = fallbackSeasonFolder ?? true;
  }

  function buildSonarrAddPayload(series) {
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

  function filterFallbackAflResults(query = fallbackAflQuery) {
    const normalized = (query || '').trim().toLowerCase();
    if (!normalized) {
      fallbackAflResults = cachedAflShows;
      return;
    }

    const tokens = normalized.split(/\s+/).filter(Boolean);
    fallbackAflResults = cachedAflShows.filter((show) => {
      const title = (show.title || '').toLowerCase();
      if (!title) return false;
      if (title.includes(normalized)) return true;
      return tokens.every((token) => title.includes(token));
    });
  }

  function scheduleFallbackAflFilter() {
    if (fallbackFilterTimeout) {
      clearTimeout(fallbackFilterTimeout);
    }
    fallbackFilterTimeout = setTimeout(() => {
      filterFallbackAflResults();
    }, 120);
  }

  async function ensureCachedAflShowsLoaded() {
    if (cachedAflShows.length > 0) {
      fallbackAflResults = cachedAflShows;
      return;
    }

    fallbackAflLoading = true;
    selectedFallbackAfl = null;
    try {
      cachedAflShows = await searchAFL('');
      fallbackAflResults = cachedAflShows;
    } catch (err) {
      cachedAflShows = [];
      fallbackAflResults = [];
      addNotification('AFL cache load failed: ' + err.message, 'error');
    } finally {
      fallbackAflLoading = false;
    }
  }

  async function goToFallbackAflStep(prefill = '') {
    addStep = ADD_STEP_FALLBACK_AFL;
    resetFallbackFlow();
    fallbackAflQuery = prefill;
    await ensureCachedAflShowsLoaded();
    filterFallbackAflResults(prefill);
  }

  async function loadSonarrLookupResults() {
    if (!sonarrLookupQuery.trim()) return;
    sonarrLookupLoading = true;
    selectedSonarrLookup = null;
    try {
      sonarrLookupResults = await lookupSonarrSeries(sonarrLookupQuery.trim());
    } catch (err) {
      sonarrLookupResults = [];
      addNotification('Sonarr lookup failed: ' + err.message, 'error');
    }
    sonarrLookupLoading = false;
  }

  async function goToFallbackSonarrStep() {
    if (!selectedFallbackAfl) return;
    sonarrLookupQuery = selectedFallbackAfl.title || '';
    ensureFallbackAddDefaults();
    addStep = ADD_STEP_FALLBACK_SONARR;
    await loadSonarrLookupResults();
  }

  function upsertSeriesLocally(series) {
    allSeries = [...allSeries.filter((item) => item.id !== series.id), series].sort((a, b) => a.title.localeCompare(b.title));
    seriesById = Object.fromEntries(allSeries.map((item) => [item.id, item]));
  }

  async function handleKeepSonarrOnly() {
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
    } catch (err) {
      addNotification('Failed to persist manual match reminder: ' + err.message, 'error');
    }
  }

  async function handleAddSelectedSonarrLookup() {
    ensureFallbackAddDefaults();
    if (!selectedFallbackAfl || !selectedSonarrLookup) return;
    sonarrAdding = true;
    try {
      const created = await addSonarrSeries(buildSonarrAddPayload(selectedSonarrLookup));
      upsertSeriesLocally(created);
      selectedSonarr = created;
      manualQuery = selectedFallbackAfl.title || created.title || '';

      const results = await searchAFL(manualQuery.trim(), created.id);
      suggestedMatches = results;
      displayedMatches = results;
      manualSearchVisible = results.length === 0 || !!created.manualMatchPending;
      const chosenResult = results.find((item) => matchSlug(item) === selectedFallbackAfl.slug);
      if (shouldAutoCreateMappingAfterSonarrAdd(chosenResult)) {
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
    } catch (err) {
      addNotification('Failed to add anime to Sonarr: ' + err.message, 'error');
    } finally {
      sonarrAdding = false;
    }
  }

  async function handleConfirmUnsafeOverride() {
    await handleConfirmMatch();
  }

  async function handlePrimaryMatchAction() {
    if (!selectedAfl) return;
    if (isUnsafeMatch(selectedAfl)) {
      addStep = ADD_STEP_UNSAFE_MATCH;
      return;
    }
    await handleConfirmMatch();
  }

  function returnToSeriesStep(prefill = '') {
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

  function matchTitle(match) {
    return match?.afl_show?.title || match?.title || '';
  }

  function matchSlug(match) {
    return match?.afl_show?.slug || match?.slug || '';
  }

  function optionState(checked) {
    return checked ? 'Included' : 'Skipped';
  }

  function optionTone(checked, variant) {
    return checked ? variant : 'secondary';
  }

  $: if (typeof document !== 'undefined') {
    document.body.style.overflow = showAddModal ? 'hidden' : '';
  }
</script>

<svelte:head>
  <title>Mizu - Anime</title>
</svelte:head>

<div class="space-y-6">
  <section class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">Anime</h1>
    </div>
    <Button onclick={openAddModal}>
      <Plus size={18} />
      Add anime
    </Button>
  </section>

  <section class="flex flex-col gap-3 sm:flex-row sm:items-center">
    <div class="relative flex-1">
      <span class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-primary">
        <Search size={20} />
      </span>
      <Input
        type="text"
        placeholder="Search anime"
        bind:value={filter}
        class="h-10 w-full pl-12"
        aria-label="Search anime"
      />
    </div>
    <div class="sm:w-48">
      <Select.Root type="single" bind:value={sortBy}>
        <Select.Trigger class="w-full">
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
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-2xl border border-border/80 bg-popover/60 text-center">
      <LoaderCircle size={30} class="animate-spin text-primary" />
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-foreground">Loading anime</h2>
        <p class="text-sm text-muted-foreground">Pulling current mappings and Sonarr state.</p>
      </div>
    </Box>
  {:else if filteredShows.length === 0 && ($shows || []).length === 0}
    <Box class="flex min-h-72 flex-col items-center justify-center gap-4 rounded-2xl border border-border/80 bg-popover/60 px-6 text-center">
      <span class="flex size-12 items-center justify-center rounded-xl border border-primary/20 bg-primary/30 text-primary">
        <Waves size={24} />
      </span>
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-foreground">No anime synced yet</h2>
        <p class="max-w-md text-sm leading-6 text-muted-foreground">Add your first Sonarr anime mapping to start monitoring filler decisions in Mizu.</p>
      </div>
      <Button onclick={openAddModal}>
        <Plus size={18} />
        Add anime
      </Button>
    </Box>
  {:else if filteredShows.length === 0}
    <Box class="flex min-h-56 flex-col items-center justify-center gap-4 rounded-2xl border border-border/80 bg-popover/60 px-6 text-center">
      <span class="flex size-12 items-center justify-center rounded-xl border border-border/80 bg-card/70 text-muted-foreground">
        <CircleAlert size={24} />
      </span>
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-foreground">No matching anime</h2>
        <p class="text-sm text-muted-foreground">Try another title, slug, or keyword.</p>
      </div>
    </Box>
  {:else}
    <section class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
      {#each filteredShows as show}
        {@const syncBadge = syncStateBadge(show.enabled)}
        <button
          type="button"
          class="group w-full cursor-pointer text-left transition-opacity duration-300 ease-in-out hover:opacity-80 focus:outline-none"
          onclick={() => openShow(show.id)}
          aria-label={`Open ${show.sonarr_title}`}
        >
          <Box class="h-full rounded-2xl border border-border/80 bg-popover/60 p-3">
            <div
              class="poster-frame relative aspect-[2/3] overflow-hidden rounded-xl border border-border/80 bg-card/80"
              data-poster-state={show.poster ? POSTER_LOADING : POSTER_FAILED}
              use:observePosterFrame={show}
            >
              {#if show.poster}
                <div class="poster-skeleton absolute inset-0" aria-hidden="true">
                  <div class="pointer-events-none absolute inset-x-0 top-0 h-28 bg-linear-to-b from-foreground/6 to-transparent"></div>
                  <div class="pointer-events-none absolute inset-0 flex items-center justify-center px-4">
                    <div class="text-center text-[4.75rem] font-black leading-none tracking-[0.24em] text-foreground/6 sm:text-[5.5rem]">
                      {createPosterMonogram(show.sonarr_title)}
                    </div>
                  </div>
                  <div class="poster-skeleton__shine"></div>
                </div>
                <img
                  src={show.poster}
                  alt={show.sonarr_title}
                  width="320"
                  height="480"
                  loading="lazy"
                  decoding="async"
                  class="poster-image h-full w-full object-cover opacity-0 transition-all duration-500 ease-in-out group-hover:scale-105"
                />
              {/if}

              <div class="poster-fallback bg-card text-3xl font-semibold tracking-tight text-muted-foreground">
                {createPosterMonogram(show.sonarr_title)}
              </div>
            </div>

            <div class="mt-3 space-y-3">
              <div>
                <h2 class="line-clamp-2 text-sm font-semibold leading-5 text-foreground">
                  {show.sonarr_title}
                </h2>
              </div>
              <div class="flex flex-wrap gap-2">
                <Badge variant="secondary">
                  <Waves size={12} />
                  {markerChip(show)}
                </Badge>
                <Badge variant={syncBadge.variant}>
                  <CircleCheckBig size={12} />
                  {syncBadge.label}
                </Badge>
              </div>
            </div>
          </Box>
        </button>
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
    <section
      class="show-add-modal-panel relative grid max-h-[calc(100vh-1.5rem)] w-full max-w-3xl gap-6 overflow-hidden rounded-2xl border border-border bg-popover p-6 shadow-[0_24px_80px_rgba(0,0,0,0.5)] sm:max-h-[calc(100vh-3rem)] sm:p-7"
      role="dialog"
      aria-modal="true"
      aria-labelledby="show-add-modal-title"
      onclick={(event) => event.stopPropagation()}
      transition:scale={{ duration: 220, start: 0.96 }}
    >
      <button
        type="button"
        class="show-add-modal-close absolute right-4 top-4 inline-flex h-9 w-9 items-center justify-center rounded-xl border border-border/80 bg-card/80 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        onclick={closeAddModal}
        aria-label="Close add anime modal"
      >
        <X size={16} />
      </button>

      <div class="space-y-7">
        <div class="space-y-3 border-b border-border/80 pb-5 pr-12">
          <h2 id="show-add-modal-title" class="text-xl font-semibold tracking-tight text-foreground">Add anime</h2>
          <!-- Step indicator -->
          <div class="flex items-center gap-1.5">
            {#each activeWizardSteps as step, i}
              {@const isCurrent = step.key === addStep}
              {@const isPast = currentStepIndex > i}
              <div class="flex items-center gap-1.5">
                <span class="flex size-5 items-center justify-center rounded-full text-xs font-medium {isCurrent ? 'bg-primary text-popover' : isPast ? 'bg-primary/20 text-primary' : 'bg-secondary text-muted-foreground'}">
                  {#if isPast}
                    <Check size={12} />
                  {:else}
                    {i + 1}
                  {/if}
                </span>
                <span class="text-xs font-medium {isCurrent ? 'text-muted-foreground' : 'text-muted-foreground'}">{step.label}</span>
                {#if i < activeWizardSteps.length - 1}
                  <span class="mx-0.5 h-px w-4 bg-secondary"></span>
                {/if}
              </div>
            {/each}
          </div>
          <p class="text-sm leading-6 text-muted-foreground">
            {#if addStep === ADD_STEP_SERIES}
              Search your actionable Sonarr anime library first. If the anime is missing, switch to cached AFL search and add it into Sonarr from here.
            {:else if addStep === ADD_STEP_FALLBACK_AFL}
              Search cached AnimeFillerList titles, then use the chosen title to look up the right Sonarr series before adding it.
            {:else if addStep === ADD_STEP_FALLBACK_SONARR}
              Confirm the Sonarr series Mizu should add for <span class="font-medium text-muted-foreground">{selectedFallbackAfl?.title}</span>.
            {:else if addStep === ADD_STEP_POST_ADD}
              <span class="font-medium text-muted-foreground">{addOutcome?.title}</span> is in Sonarr now, but still needs a safe AFL mapping before Mizu can track it.
            {:else if addStep === ADD_STEP_UNSAFE_MATCH}
              Mizu found a possible cached AFL entry for <span class="font-medium text-muted-foreground">{selectedSonarr?.title}</span>, but it needs your confirmation before creating the mapping.
            {:else}
              Match <span class="font-medium text-muted-foreground">{selectedSonarr?.title}</span> with the correct AnimeFillerList entry.
            {/if}
          </p>
        </div>

        {#if addStep === ADD_STEP_SERIES}
          {#if sonarrLoading}
            <div class="flex min-h-56 flex-col items-center justify-center gap-3 text-center">
              <LoaderCircle size={28} class="animate-spin text-primary" />
              <p class="text-sm text-muted-foreground">Loading available Sonarr anime.</p>
            </div>
          {:else if allSeries.length === 0}
            <Box class="rounded-2xl border border-border/80 bg-popover/60 p-6 text-center">
              <h3 class="text-base font-semibold text-foreground">No anime found in Sonarr</h3>
              <p class="mt-2 text-sm leading-6 text-muted-foreground">Sonarr returned no anime series for this library.</p>
            </Box>
          {:else}
            <div class="space-y-4">
              <div class="space-y-2">
                <Label for="show-add-series-search">Search</Label>
                <div class="relative">
                  <div class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-muted-foreground">
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
                <Box class="space-y-4 rounded-2xl border border-border/80 bg-popover/60 p-6 text-center">
                  <h3 class="text-base font-semibold text-foreground">No anime match that search</h3>
                  <p class="mt-2 text-sm leading-6 text-muted-foreground">Try a different title, year, or jump into the cached AFL list and add it from there.</p>
                  <div class="flex justify-center">
                    <Button variant="secondary" onclick={() => goToFallbackAflStep(seriesQuery.trim())}>
                      <Search size={16} />
                      Not in Sonarr? Search AFL and add to Sonarr
                    </Button>
                  </div>
                </Box>
              {:else}
                <ScrollArea class="h-[23rem] rounded-xl border border-border">
                  <div class="divide-y divide-border">
                    {#each filteredSeriesLibrary as series}
                      {@const seriesPoster = posterUrl(series)}
                      {@const seriesMapped = mappedSeriesIds.has(series.id)}
                      {@const manualPending = !!series.manualMatchPending}
                      {@const isSelected = selectedSonarr?.id === series.id}
                      <button
                        type="button"
                        class="group flex w-full items-center gap-3 px-4 py-3 text-left transition-colors {isSelected ? 'bg-primary/15' : seriesMapped ? 'opacity-60 hover:bg-muted/50' : 'hover:bg-muted/50'}"
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
                          class="poster-frame poster-thumb relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-md border border-border bg-card sm:w-11"
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
                          <div class="poster-fallback bg-card text-xl font-semibold tracking-tight text-muted-foreground">
                            {createPosterMonogram(series.title)}
                          </div>
                        </div>

                        <div class="min-w-0 flex-1 space-y-0.5">
                          <div class="line-clamp-1 text-sm font-medium text-foreground">{series.title}</div>
                          <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
                            <span>{series.year || 'Anime'}</span>
                            {#if series.statistics?.episodeCount}
                              <span aria-hidden="true">•</span>
                              <span>{series.statistics.episodeCount} eps</span>
                            {/if}
                            {#if manualPending}
                              <span aria-hidden="true">•</span>
                              <span class="text-warning">Needs match</span>
                            {:else if seriesMapped}
                              <span aria-hidden="true">•</span>
                              <span>Already added</span>
                            {/if}
                          </div>
                        </div>

                        {#if seriesMapped}
                          <Badge variant="secondary" class="shrink-0">Open</Badge>
                        {:else if manualPending}
                          <Badge variant="warning" class="shrink-0">Match</Badge>
                        {:else if isSelected}
                          <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                            <Check size={14} />
                          </div>
                        {:else}
                          <div class="h-7 w-7 shrink-0 rounded-full border border-border group-hover:border-primary/40"></div>
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
              <Box class="rounded-2xl border border-border/80 bg-popover/60 p-6 text-center">
                <h3 class="text-base font-semibold text-foreground">No cached AFL titles found</h3>
                <p class="mt-2 text-sm leading-6 text-muted-foreground">Try another filter or refresh the AFL cache in Settings if this anime is new.</p>
              </Box>
            {:else}
              <ScrollArea class="h-[22rem] rounded-xl border border-border">
                <div class="divide-y divide-border">
                  {#each fallbackAflResults as result}
                    {@const isSelected = selectedFallbackAfl?.slug === result.slug}
                    <button
                      type="button"
                      class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors {isSelected ? 'bg-primary/15' : 'hover:bg-muted/50'}"
                      onclick={() => (selectedFallbackAfl = result)}
                    >
                      <div class="min-w-0">
                        <div class="truncate text-sm font-medium text-foreground">{result.title}</div>
                      </div>
                      {#if isSelected}
                        <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                          <Check size={14} />
                        </div>
                      {:else}
                        <div class="h-7 w-7 shrink-0 rounded-full border border-border"></div>
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
              <Box class="rounded-2xl border border-border/80 bg-popover/60 p-6 text-center">
                <h3 class="text-base font-semibold text-foreground">No Sonarr results found</h3>
                <p class="mt-2 text-sm leading-6 text-muted-foreground">Try another title variation for {selectedFallbackAfl?.title} and search again.</p>
              </Box>
            {:else}
              <ScrollArea class="h-[18rem] rounded-xl border border-border">
                <div class="divide-y divide-border">
                  {#each sonarrLookupResults as result}
                    {@const lookupPoster = posterUrl(result)}
                    {@const isSelected = selectedSonarrLookup?.tvdbId === result.tvdbId && selectedSonarrLookup?.title === result.title}
                    <button
                      type="button"
                      class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors {isSelected ? 'bg-primary/15' : 'hover:bg-muted/50'}"
                      onclick={() => (selectedSonarrLookup = result)}
                    >
                      <div
                        class="poster-frame poster-thumb relative aspect-[2/3] w-10 shrink-0 overflow-hidden rounded-md border border-border bg-card sm:w-11"
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
                        <div class="poster-fallback bg-card text-xl font-semibold tracking-tight text-muted-foreground">
                          {createPosterMonogram(result.title)}
                        </div>
                      </div>

                      <div class="min-w-0 flex-1 space-y-0.5">
                        <div class="truncate text-sm font-medium text-foreground">{result.title}</div>
                        <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
                          <span>{result.year || 'Unknown year'}</span>
                          {#if result.network}
                            <span aria-hidden="true">•</span>
                            <span>{result.network}</span>
                          {/if}
                          {#if result.status}
                            <span aria-hidden="true">•</span>
                            <span>{result.status}</span>
                          {/if}
                        </div>
                      </div>

                      {#if isSelected}
                        <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                          <Check size={14} />
                        </div>
                      {:else}
                        <div class="h-7 w-7 shrink-0 rounded-full border border-border"></div>
                      {/if}
                    </button>
                  {/each}
                </div>
              </ScrollArea>
            {/if}

            <Box class="grid gap-3 rounded-2xl border border-border/80 bg-popover/70 p-4 md:grid-cols-[minmax(0,1fr)_18rem] md:items-start">
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

              <label class="flex items-center justify-between gap-4 rounded-xl border border-border/80 bg-popover/70 px-4 py-3">
                <div class="text-sm font-medium text-foreground">Season folder</div>
                <Checkbox bind:checked={fallbackSeasonFolder} />
              </label>

              {#if sonarrAddOptionsError}
                <div class="rounded-xl border border-warning/20 bg-warning/10 px-4 py-3 text-sm text-warning">
                  Unable to load Sonarr quality profiles right now: {sonarrAddOptionsError}
                </div>
              {/if}
            </Box>
          </div>
        {:else if addStep === ADD_STEP_UNSAFE_MATCH}
          <Box class="space-y-4 rounded-2xl border border-warning/20 bg-warning/10 p-5">
            <div class="flex items-start gap-3">
              <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-warning/30 bg-warning/10 text-warning">
                <CircleAlert size={16} />
              </span>
              <div class="space-y-2">
                <h3 class="text-base font-semibold text-foreground">This cached AFL match needs confirmation</h3>
                <p class="text-sm leading-6 text-warning/85">
                  Mizu could not safely verify that <span class="font-medium text-foreground">{matchTitle(selectedAfl)}</span> is the right AnimeFillerList entry for
                  <span class="font-medium text-foreground"> {selectedSonarr?.title}</span>. You can still create the mapping manually, or keep the series in Sonarr only and match it later.
                </p>
              </div>
            </div>

            <div class="rounded-xl border border-border/80 bg-popover/70 p-4">
              <div class="text-sm font-semibold text-foreground">{matchTitle(selectedAfl)}</div>
              <div class="mt-1 text-sm text-muted-foreground">{matchSlug(selectedAfl)}</div>
            </div>
          </Box>
        {:else if addStep === ADD_STEP_POST_ADD}
          <Box class="space-y-4 rounded-2xl border border-border/80 bg-popover/70 p-5">
            <div class="flex items-start gap-3">
              <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-primary/30 bg-primary/40 text-primary">
                <CircleCheckBig size={16} />
              </span>
              <div class="space-y-2">
                <h3 class="text-base font-semibold text-foreground">Added to Sonarr</h3>
                <p class="text-sm leading-6 text-muted-foreground">
                  {addOutcome?.title || selectedSonarr?.title} is in Sonarr now. Mizu did not create a mapping because it could not confirm a safe AFL match, so this anime will stay visible in the add modal as <span class="font-medium text-foreground">Needs match</span>.
                </p>
              </div>
            </div>
          </Box>
        {:else}
          {#if manualSearchVisible}
            <div class="space-y-3 rounded-2xl border border-border/80 bg-popover/60 p-4">
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
            <Box class="rounded-2xl border border-border/80 bg-popover/60 p-6 text-center">
              <h3 class="text-base font-semibold text-foreground">No matches found</h3>
              <p class="mt-2 text-sm leading-6 text-muted-foreground">Refresh the AnimeFillerList cache in Settings if this anime is new, then try again.</p>
            </Box>
          {:else if selectedAfl && isCompactMatchSelection(selectedAfl)}
            <Box class="space-y-4 rounded-2xl border border-border/80 bg-popover/70 p-5">
              <div class="space-y-2">
                <div class="text-sm font-semibold text-foreground">{matchTitle(selectedAfl)}</div>
                <div class="flex flex-wrap gap-2">
                  <Badge variant="info">{Math.round((selectedAfl.confidence || 0) * 100)}% confidence</Badge>
                  <Badge variant="secondary">{selectedAfl.method}</Badge>
                </div>
                <p class="text-sm leading-6 text-muted-foreground">Looks like an exact cached AFL match, so this step is just a quick confirmation before Mizu creates the mapping.</p>
              </div>
            </Box>
          {:else}
            <ScrollArea class="h-[18rem] rounded-xl border border-border">
              <div class="divide-y divide-border">
                {#each displayedMatches as result}
                  {@const isSelected = selectedAfl && matchSlug(selectedAfl) === matchSlug(result)}
                  <button
                    type="button"
                    class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors {isSelected ? 'bg-primary/15' : 'hover:bg-muted/50'}"
                    onclick={() => (selectedAfl = result)}
                  >
                    <div class="min-w-0 space-y-1">
                      <div class="truncate text-sm font-medium text-foreground">{matchTitle(result)}</div>
                      <div class="truncate text-xs text-muted-foreground">{matchSlug(result)}</div>
                      <div class="flex flex-wrap gap-2">
                        {#if result.confidence != null}
                          <Badge variant="info">{Math.round(result.confidence * 100)}% confidence</Badge>
                          <Badge variant="secondary">{result.method}</Badge>
                        {:else}
                          <Badge variant="warning">Manual review</Badge>
                        {/if}
                      </div>
                    </div>
                    {#if isSelected}
                      <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground">
                        <Check size={14} />
                      </div>
                    {:else}
                      <div class="h-7 w-7 shrink-0 rounded-full border border-border"></div>
                    {/if}
                  </button>
                {/each}
              </div>
            </ScrollArea>
          {/if}

          {#if selectedAfl}
            <Box class="space-y-4 rounded-2xl border border-border/80 bg-popover/70 p-4">
              <div class="space-y-1">
                <h3 class="text-base font-semibold text-foreground">Default tracking rules</h3>
                <p class="text-sm text-muted-foreground">Set the initial monitoring rules for this mapping before it is created.</p>
              </div>

              <div class="grid gap-3 md:grid-cols-3">
                <label class="flex items-start gap-3 rounded-xl border border-border/80 bg-popover/70 p-4">
                  <Checkbox bind:checked={addTrackAnimeCanon} />
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-foreground">Anime canon</div>
                    <Badge variant={optionTone(addTrackAnimeCanon, 'default')}>{optionState(addTrackAnimeCanon)}</Badge>
                  </div>
                </label>

                <label class="flex items-start gap-3 rounded-xl border border-border/80 bg-popover/70 p-4">
                  <Checkbox bind:checked={addTrackMixed} />
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-foreground">Mixed canon</div>
                    <Badge variant={optionTone(addTrackMixed, 'warning')}>{optionState(addTrackMixed)}</Badge>
                  </div>
                </label>

                <label class="flex items-start gap-3 rounded-xl border border-border/80 bg-popover/70 p-4">
                  <Checkbox bind:checked={addTrackFiller} />
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-foreground">Filler</div>
                    <Badge variant={optionTone(addTrackFiller, 'destructive')}>{optionState(addTrackFiller)}</Badge>
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
            </Box>
          {/if}
        {/if}

        <div class="flex flex-col-reverse gap-3 border-t border-border/80 pt-5 sm:flex-row sm:justify-between">
          <div class="flex flex-col-reverse gap-3 sm:flex-row">
            <Button variant="secondary" onclick={closeAddModal}>Cancel</Button>
            {#if addStep === ADD_STEP_FALLBACK_AFL}
              <Button variant="ghost" onclick={() => returnToSeriesStep()}>Back to Sonarr</Button>
            {:else if addStep === ADD_STEP_FALLBACK_SONARR}
              <Button variant="ghost" onclick={() => (addStep = ADD_STEP_FALLBACK_AFL)}>Back to AFL</Button>
            {:else if addStep === ADD_STEP_MATCH}
              <Button variant="ghost" onclick={() => returnToSeriesStep(selectedSonarr?.title || '')}>Change anime</Button>
            {:else if addStep === ADD_STEP_UNSAFE_MATCH}
              <Button variant="ghost" onclick={() => (addStep = ADD_STEP_MATCH)}>Back to matches</Button>
            {:else if addStep === ADD_STEP_POST_ADD}
              <Button variant="ghost" onclick={() => returnToSeriesStep(addOutcome?.title || '')}>Back to library</Button>
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
              <Button onclick={closeAddModal}>
                <Check size={16} />
                Done
              </Button>
            {/if}
          </div>
        </div>
      </div>
    </section>
  </div>
{/if}
