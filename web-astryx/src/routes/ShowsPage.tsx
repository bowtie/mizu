import { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { EmptyState } from '@astryxdesign/core/EmptyState';
import { Dialog } from '@astryxdesign/core/Dialog';
import { TextInput } from '@astryxdesign/core/TextInput';
import { Selector } from '@astryxdesign/core/Selector';
import { Switch } from '@astryxdesign/core/Switch';
import { List, ListItem } from '@astryxdesign/core/List';
import { Divider } from '@astryxdesign/core/Divider';
import { Plus, Search, Waves, CircleAlert, Check, X } from 'lucide-react';
import { useAppStore } from '../lib/store';
import {
  addSonarrSeries,
  confirmMatch,
  getShows,
  getSettings,
  getSonarrAddOptions,
  getSonarrSeries,
  lookupSonarrSeries,
  markManualMatchPending,
  searchAFL,
  suggestAFL,
} from '../lib/api';
import type { AFLMatch, SelectOption, Settings, Show, SonarrAddOptions, SonarrAddPayload, SonarrSeries } from '../lib/types';
import { resolveSonarrImageUrl } from '../lib/sonarr-images';
import { syncStateBadge } from '../lib/fox';

const START_FROM_BEGINNING = 'from_beginning';
const START_FROM_CURRENT = 'from_current_progress';
const ADD_STEP_SERIES = 'series';
const ADD_STEP_FALLBACK_AFL = 'fallback_afl';
const ADD_STEP_FALLBACK_SONARR = 'fallback_sonarr';
const ADD_STEP_MATCH = 'match';
const ADD_STEP_UNSAFE_MATCH = 'unsafe_match';
const ADD_STEP_POST_ADD = 'post_add';

interface AddOutcome {
  title: string;
  type: string;
  mappingId?: string | number | null;
}

const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean =>
  (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

const sortOptions: SelectOption[] = [
  { value: 'title', label: 'Title (A-Z)' },
  { value: 'title_desc', label: 'Title (Z-A)' },
  { value: 'watermark', label: 'Latest progress' },
];

const startModeOptions: SelectOption[] = [
  { value: START_FROM_CURRENT, label: 'Current progress' },
  { value: START_FROM_BEGINNING, label: 'Episode 1' },
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

function isUnsafeMatch(match: AFLMatch | null): boolean {
  return !!match && match.confidence == null;
}
function isCompactMatchSelection(match: AFLMatch | null): boolean {
  if (!match || isUnsafeMatch(match)) return false;
  return match.method === 'exact' || match.method === 'normalized' || (match.confidence ?? 0) >= 0.95;
}
function shouldAutoCreateMappingAfterSonarrAdd(match: AFLMatch | null): boolean {
  return !!match && !isUnsafeMatch(match) && match.method === 'exact' && (match.confidence ?? 0) >= 1;
}
function matchTitle(match: AFLMatch | null | undefined): string {
  return match?.afl_show?.title || match?.title || '';
}
function matchSlug(match: AFLMatch | null | undefined): string {
  return match?.afl_show?.slug || match?.slug || '';
}

export function ShowsPage() {
  const navigate = useNavigate();
  const { shows, setShows, settings, setSettings, addNotification } = useAppStore();

  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [sortBy, setSortBy] = useState('title');

  const [allSeries, setAllSeries] = useState<SonarrSeries[]>([]);
  const [addSeriesLibrary, setAddSeriesLibrary] = useState<SonarrSeries[]>([]);
  const [seriesById, setSeriesById] = useState<Record<string | number, SonarrSeries>>({});

  const [showAddModal, setShowAddModal] = useState(false);
  const [addStep, setAddStep] = useState(ADD_STEP_SERIES);
  const [sonarrLoading, setSonarrLoading] = useState(false);
  const [seriesQuery, setSeriesQuery] = useState('');
  const [selectedSonarr, setSelectedSonarr] = useState<SonarrSeries | null>(null);
  const [displayedMatches, setDisplayedMatches] = useState<AFLMatch[]>([]);
  const [matchLoading, setMatchLoading] = useState(false);
  const [manualQuery, setManualQuery] = useState('');
  const [manualSearching, setManualSearching] = useState(false);
  const [manualSearchVisible, setManualSearchVisible] = useState(false);
  const [selectedAfl, setSelectedAfl] = useState<AFLMatch | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [addTrackFiller, setAddTrackFiller] = useState(false);
  const [addTrackMixed, setAddTrackMixed] = useState(true);
  const [addTrackAnimeCanon, setAddTrackAnimeCanon] = useState(true);
  const [addStartMode, setAddStartMode] = useState(START_FROM_CURRENT);
  const [fallbackAflQuery, setFallbackAflQuery] = useState('');
  const [cachedAflShows, setCachedAflShows] = useState<AFLMatch[]>([]);
  const [fallbackAflResults, setFallbackAflResults] = useState<AFLMatch[]>([]);
  const [fallbackAflLoading, setFallbackAflLoading] = useState(false);
  const [selectedFallbackAfl, setSelectedFallbackAfl] = useState<AFLMatch | null>(null);
  const [sonarrLookupQuery, setSonarrLookupQuery] = useState('');
  const [sonarrLookupResults, setSonarrLookupResults] = useState<SonarrSeries[]>([]);
  const [sonarrLookupLoading, setSonarrLookupLoading] = useState(false);
  const [selectedSonarrLookup, setSelectedSonarrLookup] = useState<SonarrSeries | null>(null);
  const [sonarrAdding, setSonarrAdding] = useState(false);
  const [sonarrRootFolderOptions, setSonarrRootFolderOptions] = useState<SelectOption[]>([]);
  const [sonarrQualityProfileOptions, setSonarrQualityProfileOptions] = useState<SelectOption[]>([]);
  const [fallbackQualityProfileId, setFallbackQualityProfileId] = useState('');
  const [fallbackSeasonFolder, setFallbackSeasonFolder] = useState(true);
  const [addOutcome, setAddOutcome] = useState<AddOutcome | null>(null);
  const fallbackFilterTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);

  const mappedSeriesIds = new Set((shows || []).map((s: Show) => s.sonarr_series_id));

  const mappedShows = (shows || []).map((show: Show) => {
    const series = seriesById[show.sonarr_series_id] || null;
    return {
      ...show,
      poster: (settings?.enable_anilist_artwork && show.anilist_poster_url) ? show.anilist_poster_url : posterUrl(series),
    };
  });

  const filteredShows = mappedShows
    .filter((show: Show & { poster?: string }) => {
      if (!filter) return true;
      const q = filter.toLowerCase();
      return (show.sonarr_title || '').toLowerCase().includes(q) || (show.afl_title || show.afl_slug || '').toLowerCase().includes(q);
    })
    .sort((a: Show, b: Show) => {
      if (sortBy === 'title') return (a.sonarr_title || '').localeCompare(b.sonarr_title || '');
      if (sortBy === 'title_desc') return (b.sonarr_title || '').localeCompare(a.sonarr_title || '');
      if (sortBy === 'watermark') return (b.sync_watermark || 0) - (a.sync_watermark || 0);
      return 0;
    });

  const filteredSeriesLibrary = addSeriesLibrary
    .filter((series: SonarrSeries) => {
      const q = seriesQuery.trim().toLowerCase();
      if (!q) return true;
      return (series.title || '').toLowerCase().includes(q) || String(series.year || '').includes(q);
    })
    .sort((a: SonarrSeries, b: SonarrSeries) => {
      const aMapped = mappedSeriesIds.has(a.id) ? 1 : 0;
      const bMapped = mappedSeriesIds.has(b.id) ? 1 : 0;
      if (aMapped !== bMapped) return aMapped - bMapped;
      return a.title.localeCompare(b.title);
    });

  const activeWizardSteps = [ADD_STEP_FALLBACK_AFL, ADD_STEP_FALLBACK_SONARR].includes(addStep) ? wizardFallbackSteps : wizardSteps;
  const currentStepIndex = activeWizardSteps.findIndex((s) => s.key === addStep);

  function posterUrl(series: SonarrSeries | null | undefined): string {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((img) => img.coverType === 'poster') || series.images.find((img) => img.coverType === 'fanart') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, settings?.sonarr_url || '');
  }

  function bannerUrl(show: Show, series: SonarrSeries | null | undefined): string {
    if (settings?.enable_anilist_artwork && show.anilist_banner_url) return show.anilist_banner_url;
    if (!series?.images?.length) return '';
    const preferred = series.images.find((img) => img.coverType === 'fanart') || series.images.find((img) => img.coverType === 'banner') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, settings?.sonarr_url || '');
  }
  void bannerUrl;

  const loadLanding = useCallback(async () => {
    setLoading(true);
    try {
      const [showsRes, seriesRes, settingsRes] = await Promise.allSettled([getShows(), getSonarrSeries(), getSettings()]);
      const nextShows: Show[] = showsRes.status === 'fulfilled' ? (showsRes.value || []) : [];
      setShows(nextShows);
      const nextSeries: SonarrSeries[] = seriesRes.status === 'fulfilled' ? (seriesRes.value || []) : [];
      setAllSeries(nextSeries);
      setSeriesById(Object.fromEntries(nextSeries.map((s) => [s.id, s])));
      if (settingsRes.status === 'fulfilled') {
        setSettings(settingsRes.value ?? null);
      }
    } catch (err: any) {
      addNotification('Failed to load Mizu anime: ' + err.message, 'error');
    }
    setLoading(false);
  }, [setShows, setSettings, addNotification]);

  useEffect(() => {
    loadLanding();
  }, [loadLanding]);

  useEffect(() => {
    return () => {
      if (fallbackFilterTimeout.current) clearTimeout(fallbackFilterTimeout.current);
    };
  }, []);

  const refreshSettings = async () => {
    try {
      const current = await getSettings();
      setSettings(current);
    } catch (err: any) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
  };

  const resetAddRuleDefaults = (current: Settings | null = settings) => {
    setAddTrackFiller(actionToTracked(current?.default_filler_action, false));
    setAddTrackMixed(actionToTracked(current?.default_mixed_canon_action, true));
    setAddTrackAnimeCanon(actionToTracked(current?.default_anime_canon_action, true));
    setAddStartMode(START_FROM_CURRENT);
  };

  const loadSonarrAddOptions = async () => {
    try {
      const data: SonarrAddOptions = await getSonarrAddOptions();
      setSonarrRootFolderOptions((data.root_folders || []).map((f) => ({ value: f.path, label: f.path })));
      setSonarrQualityProfileOptions((data.quality_profiles || []).map((p) => ({ value: String(p.id), label: p.name })));
      if (!fallbackQualityProfileId) {
        setFallbackQualityProfileId(preferredAnimeQualityProfileId());
      }
    } catch {
      // silent
    }
  };

  const preferredAnimeRootFolder = (): string => {
    const counts = new Map<string, number>();
    let bestPath = '';
    let bestCount = 0;
    for (const series of allSeries) {
      const path = (series?.rootFolderPath || '').trim();
      if (!path) continue;
      const next = (counts.get(path) || 0) + 1;
      counts.set(path, next);
      if (next > bestCount) { bestCount = next; bestPath = path; }
    }
    return bestPath || sonarrRootFolderOptions[0]?.value || '';
  };

  const preferredAnimeQualityProfileId = (): string => {
    const counts = new Map<string, number>();
    let bestValue = '';
    let bestCount = 0;
    for (const series of allSeries) {
      const value = Number(series?.qualityProfileId || 0);
      if (!value) continue;
      const key = String(value);
      const next = (counts.get(key) || 0) + 1;
      counts.set(key, next);
      if (next > bestCount) { bestCount = next; bestValue = key; }
    }
    return bestValue || sonarrQualityProfileOptions[0]?.value || '';
  };

  const resetFallbackFlow = () => {
    setFallbackAflQuery('');
    setFallbackAflResults(cachedAflShows);
    setSelectedFallbackAfl(null);
    setSonarrLookupQuery('');
    setSonarrLookupResults([]);
    setSelectedSonarrLookup(null);
    setFallbackQualityProfileId('');
    setFallbackSeasonFolder(true);
    setAddOutcome(null);
    setSelectedAfl(null);
  };

  const openAddModal = async () => {
    setAddStep(ADD_STEP_SERIES);
    setSeriesQuery('');
    setSelectedSonarr(null);
    setDisplayedMatches([]);
    setSelectedAfl(null);
    setManualQuery('');
    setManualSearchVisible(false);
    resetFallbackFlow();
    setShowAddModal(true);

    await Promise.all([refreshSettings(), loadSonarrAddOptions()]);
    resetAddRuleDefaults(settings);

    setSonarrLoading(true);
    try {
      const [fullSeries, modalSeries] = await Promise.all([
        allSeries.length ? Promise.resolve(allSeries) : getSonarrSeries(),
        getSonarrSeries(true),
      ]);
      setAllSeries(fullSeries || []);
      setAddSeriesLibrary(modalSeries || []);
      setSeriesById(Object.fromEntries((fullSeries || []).map((s) => [s.id, s])));
    } catch (err: any) {
      addNotification('Failed to load Sonarr series: ' + err.message, 'error');
    }
    setSonarrLoading(false);
  };

  const openShow = (showId: string | number) => {
    navigate(`/shows/${showId}`);
  };

  const openExistingMapping = (seriesId: string | number) => {
    const mapped = (shows || []).find((s: Show) => s.sonarr_series_id === seriesId);
    if (!mapped) return;
    setShowAddModal(false);
    openShow(mapped.id);
  };

  const loadSuggestedMatches = async () => {
    if (!selectedSonarr) return;
    setMatchLoading(true);
    setSelectedAfl(null);
    try {
      const suggested = await suggestAFL(selectedSonarr.id);
      setDisplayedMatches(suggested);
      setManualSearchVisible(suggested.length === 0 || !!selectedSonarr.manualMatchPending);
      const bestMatch = suggested[0];
      if (bestMatch && isCompactMatchSelection(bestMatch)) {
        setSelectedAfl(bestMatch);
      }
      if (manualSearchVisible && selectedSonarr.manualMatchPending && manualQuery.trim()) {
        setDisplayedMatches(await searchAFL(manualQuery.trim(), selectedSonarr.id));
      }
    } catch {
      setDisplayedMatches([]);
      setManualSearchVisible(true);
      addNotification('Failed to load AFL suggestions', 'error');
    }
    setMatchLoading(false);
  };

  const goToMatchStep = async () => {
    if (!selectedSonarr) return;
    setAddStep(ADD_STEP_MATCH);
    setManualQuery(selectedSonarr.title || '');
    await loadSuggestedMatches();
  };

  const handleManualSearch = async () => {
    if (!selectedSonarr || !manualSearchVisible) return;
    setManualSearching(true);
    setSelectedAfl(null);
    try {
      setDisplayedMatches(await searchAFL(manualQuery.trim(), selectedSonarr.id));
    } catch {
      setDisplayedMatches([]);
      addNotification('AFL search failed', 'error');
    }
    setManualSearching(false);
  };

  const handleConfirmMatch = async () => {
    if (!selectedSonarr || !selectedAfl) return;
    setConfirming(true);
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
      addNotification(`Added "${selectedSonarr.title}" to Mizu`, 'success');
      setAddOutcome({ title: selectedSonarr.title, type: 'mapping_created', mappingId: created?.id || null });
      await loadLanding();
      setAddStep(ADD_STEP_POST_ADD);
    } catch (err: any) {
      addNotification('Failed to sync anime: ' + err.message, 'error');
    } finally {
      setConfirming(false);
    }
  };

  const handlePrimaryMatchAction = async () => {
    if (!selectedAfl) return;
    if (isUnsafeMatch(selectedAfl)) {
      setAddStep(ADD_STEP_UNSAFE_MATCH);
      return;
    }
    await handleConfirmMatch();
  };

  const handleKeepSonarrOnly = async () => {
    if (!selectedSonarr?.id) return;
    try {
      await markManualMatchPending(selectedSonarr.id);
      setAddOutcome({ title: selectedSonarr.title, type: 'pending_manual_match' });
      setSelectedSonarr({ ...selectedSonarr, manualMatchPending: true });
      setAddStep(ADD_STEP_POST_ADD);
    } catch (err: any) {
      addNotification('Failed to persist manual match reminder: ' + err.message, 'error');
    }
  };

  const ensureCachedAflShowsLoaded = async () => {
    if (cachedAflShows.length > 0) {
      setFallbackAflResults(cachedAflShows);
      return;
    }
    setFallbackAflLoading(true);
    setSelectedFallbackAfl(null);
    try {
      const cached = await searchAFL('');
      setCachedAflShows(cached);
      setFallbackAflResults(cached);
    } catch (err: any) {
      setCachedAflShows([]);
      setFallbackAflResults([]);
      addNotification('AFL cache load failed: ' + err.message, 'error');
    } finally {
      setFallbackAflLoading(false);
    }
  };

  const filterFallbackAflResults = (query: string = fallbackAflQuery) => {
    const normalized = (query || '').trim().toLowerCase();
    if (!normalized) {
      setFallbackAflResults(cachedAflShows);
      return;
    }
    const tokens = normalized.split(/\s+/).filter(Boolean);
    setFallbackAflResults(cachedAflShows.filter((show) => {
      const title = (show.title || '').toLowerCase();
      if (!title) return false;
      if (title.includes(normalized)) return true;
      return tokens.every((token) => title.includes(token));
    }));
  };

  const scheduleFallbackAflFilter = () => {
    if (fallbackFilterTimeout.current) clearTimeout(fallbackFilterTimeout.current);
    fallbackFilterTimeout.current = setTimeout(() => filterFallbackAflResults(), 120);
  };

  const goToFallbackAflStep = async (prefill: string = '') => {
    setAddStep(ADD_STEP_FALLBACK_AFL);
    resetFallbackFlow();
    setFallbackAflQuery(prefill);
    await ensureCachedAflShowsLoaded();
    filterFallbackAflResults(prefill);
  };

  const loadSonarrLookupResults = async () => {
    if (!sonarrLookupQuery.trim()) return;
    setSonarrLookupLoading(true);
    setSelectedSonarrLookup(null);
    try {
      setSonarrLookupResults(await lookupSonarrSeries(sonarrLookupQuery.trim()));
    } catch (err: any) {
      setSonarrLookupResults([]);
      addNotification('Sonarr lookup failed: ' + err.message, 'error');
    }
    setSonarrLookupLoading(false);
  };

  const goToFallbackSonarrStep = async () => {
    if (!selectedFallbackAfl) return;
    setSonarrLookupQuery(selectedFallbackAfl.title || '');
    setAddStep(ADD_STEP_FALLBACK_SONARR);
    await loadSonarrLookupResults();
  };

  const buildSonarrAddPayload = (series: SonarrSeries): SonarrAddPayload => ({
    series: { ...series, seriesType: 'anime' },
    root_folder_path: preferredAnimeRootFolder(),
    quality_profile_id: Number(fallbackQualityProfileId || 0),
    monitor_mode: 'none',
    season_folder: fallbackSeasonFolder,
    search_on_add: false,
  });

  const handleAddSelectedSonarrLookup = async () => {
    if (!selectedFallbackAfl || !selectedSonarrLookup) return;
    const fallbackAflSlug = selectedFallbackAfl.slug;
    setSonarrAdding(true);
    try {
      const created = await addSonarrSeries(buildSonarrAddPayload(selectedSonarrLookup));
      const nextAll = [...allSeries.filter((s) => s.id !== created.id), created].sort((a, b) => a.title.localeCompare(b.title));
      setAllSeries(nextAll);
      setSeriesById(Object.fromEntries(nextAll.map((s) => [s.id, s])));
      setSelectedSonarr(created);
      setManualQuery(selectedFallbackAfl.title || created.title || '');

      const results = await searchAFL(manualQuery.trim(), created.id);
      setDisplayedMatches(results);
      setManualSearchVisible(results.length === 0 || !!created.manualMatchPending);
      const chosenResult = results.find((item) => matchSlug(item) === fallbackAflSlug);
      if (chosenResult && shouldAutoCreateMappingAfterSonarrAdd(chosenResult)) {
        setSelectedAfl(chosenResult);
        await handleConfirmMatch();
        return;
      }
      if (chosenResult && !isUnsafeMatch(chosenResult)) {
        setSelectedAfl(chosenResult);
        setAddStep(ADD_STEP_MATCH);
        return;
      }
      if (chosenResult) {
        setSelectedAfl(chosenResult);
        setAddStep(ADD_STEP_UNSAFE_MATCH);
        return;
      }
      await handleKeepSonarrOnly();
    } catch (err: any) {
      addNotification('Failed to add anime to Sonarr: ' + err.message, 'error');
    } finally {
      setSonarrAdding(false);
    }
  };

  const returnToSeriesStep = (prefill: string = '') => {
    setAddStep(ADD_STEP_SERIES);
    setAddOutcome(null);
    setSelectedAfl(null);
    setSelectedFallbackAfl(null);
    setSelectedSonarrLookup(null);
    setSonarrLookupResults([]);
    setFallbackAflResults([]);
    if (prefill) setSeriesQuery(prefill);
  };

  const stepDescription = (): string => {
    if (addStep === ADD_STEP_SERIES) return 'Search your actionable Sonarr anime library first. If the anime is missing, switch to cached AFL search and add it into Sonarr from here.';
    if (addStep === ADD_STEP_FALLBACK_AFL) return 'Search cached AnimeFillerList titles, then use the chosen title to look up the right Sonarr series before adding it.';
    if (addStep === ADD_STEP_FALLBACK_SONARR) return `Confirm the Sonarr series Mizu should add for ${selectedFallbackAfl?.title || ''}.`;
    if (addStep === ADD_STEP_POST_ADD) {
      if (addOutcome?.type === 'mapping_created') return `${addOutcome?.title} is now tracked by Mizu.`;
      return `${addOutcome?.title} is in Sonarr now, but still needs a safe AFL mapping before Mizu can track it.`;
    }
    if (addStep === ADD_STEP_UNSAFE_MATCH) return `Mizu found a possible cached AFL entry for ${selectedSonarr?.title || ''}, but it needs your confirmation before creating the mapping.`;
    return `Match ${selectedSonarr?.title || ''} with the correct AnimeFillerList entry.`;
  };

  return (
    <Section padding={6}>
      <VStack gap={6}>
        {/* Header */}
        <HStack justify="between" align="end" wrap="wrap">
          <VStack gap={1}>
            <Heading level={1}>Anime</Heading>
            <Text type="supporting">
              {!loading && <><Text type="body" hasTabularNumbers>{(shows || []).length}</Text> tracked · </>}
              Tracked series, sync status, and filler mapping.
            </Text>
          </VStack>
          <Button label="Add anime" variant="primary" icon={<Plus size={16} />} onClick={openAddModal} />
        </HStack>

        {/* Search + sort */}
        <HStack gap={3} align="center" wrap="wrap">
          <TextInput
            label="Search anime"
            value={filter}
            onChange={setFilter}
            placeholder="Search anime by title or slug"
            isLabelHidden
            startIcon={Search}
            width="100%"
          />
          <Selector
            label="Sort by"
            options={sortOptions}
            value={sortBy}
            onChange={setSortBy}
            width={200}
          />
        </HStack>

        {/* Content */}
        {loading ? (
          <VStack gap={3} hAlign="center" padding={10}>
            <Spinner size="lg" label="Loading anime" />
          </VStack>
        ) : filteredShows.length === 0 && (shows || []).length === 0 ? (
          <EmptyState
            title="No anime synced yet"
            description="Add your first Sonarr anime mapping to start monitoring filler decisions in Mizu."
            icon={<Waves size={18} />}
            actions={<Button label="Add anime" variant="primary" icon={<Plus size={16} />} onClick={openAddModal} />}
          />
        ) : filteredShows.length === 0 ? (
          <EmptyState
            title="No matching anime"
            description="Try another title, slug, or keyword."
            icon={<CircleAlert size={18} />}
          />
        ) : (
          <VStack gap={0}>
            <HStack gap={3} align="center">
              <Text type="supporting">Tracked</Text>
              <Divider />
            </HStack>
            <List hasDividers>
              {filteredShows.map((show: Show & { poster?: string }) => {
                const badge = syncStateBadge(show.enabled);
                return (
                  <ListItem
                    key={String(show.id)}
                    label={show.sonarr_title}
                    description={`#${show.sync_watermark > 0 ? show.sync_watermark : 0}`}
                    onClick={() => openShow(show.id)}
                    startContent={
                      show.poster ? (
                        <img
                          src={show.poster}
                          alt=""
                          loading="lazy"
                          style={{ width: 40, height: 56, objectFit: 'cover', borderRadius: 'var(--radius-2)' }}
                        />
                      ) : undefined
                    }
                    endContent={
                      <Badge variant={show.enabled ? 'success' : 'neutral'} label={badge.label} />
                    }
                  />
                );
              })}
            </List>
          </VStack>
        )}
      </VStack>

      {/* Add anime dialog */}
      <Dialog
        isOpen={showAddModal}
        onOpenChange={setShowAddModal}
        width={640}
        maxHeight="85vh"
        purpose="form"
      >
        <VStack gap={4} padding={6} isScrollable height="100%">
          <HStack justify="between" align="center">
            <Heading level={2}>Add anime</Heading>
            <Button label="Close" variant="ghost" isIconOnly icon={<X size={16} />} onClick={() => setShowAddModal(false)} />
          </HStack>

          {/* Step progress */}
          <HStack gap={2} align="center" wrap="wrap">
            {activeWizardSteps.map((step, i) => {
              const isCurrent = step.key === addStep;
              const isPast = currentStepIndex > i;
              return (
                <HStack key={step.key} gap={1} align="center">
                  {i > 0 && <Divider />}
                  {isPast && <Check size={10} color="var(--color-accent)" />}
                  <Text type="supporting" color={isCurrent ? 'primary' : isPast ? 'secondary' : 'placeholder'}>{step.label}</Text>
                </HStack>
              );
            })}
          </HStack>

          <Text type="supporting">{stepDescription()}</Text>

          <Divider />

          {/* Step content */}
          {addStep === ADD_STEP_SERIES && (
            sonarrLoading ? (
              <VStack gap={3} hAlign="center" padding={10}>
                <Spinner size="lg" label="Loading available Sonarr anime" />
              </VStack>
            ) : allSeries.length === 0 ? (
              <EmptyState title="No anime found in Sonarr" description="Sonarr returned no anime series for this library." />
            ) : (
              <VStack gap={4}>
                <TextInput
                  label="Search"
                  value={seriesQuery}
                  onChange={setSeriesQuery}
                  placeholder="Search Sonarr anime"
                  startIcon={Search}
                />
                {filteredSeriesLibrary.length === 0 ? (
                  <EmptyState
                    title="No anime match that search"
                    description="Try a different title, year, or jump into the cached AFL list and add it from there."
                    actions={<Button label="Not in Sonarr? Search AFL and add to Sonarr" variant="secondary" icon={<Search size={16} />} onClick={() => goToFallbackAflStep(seriesQuery.trim())} />}
                  />
                ) : (
                  <List hasDividers style={{ maxHeight: 'var(--spacing-80)', overflow: 'auto' }}>
                    {filteredSeriesLibrary.map((series) => {
                      const seriesMapped = mappedSeriesIds.has(series.id);
                      const manualPending = !!series.manualMatchPending;
                      const isSelected = selectedSonarr?.id === series.id;
                      return (
                        <ListItem
                          key={series.id}
                          label={series.title}
                          description={`${series.year || 'Anime'}${series.statistics?.episodeCount ? ` · ${series.statistics.episodeCount} eps` : ''}${manualPending ? ' · Needs match' : seriesMapped ? ' · Already added' : ''}`}
                          onClick={() => {
                            if (seriesMapped) { openExistingMapping(series.id); return; }
                            setSelectedSonarr(series);
                          }}
                          startContent={
                            <img
                              src={posterUrl(series)}
                              alt=""
                              loading="lazy"
                              style={{ width: 32, height: 44, objectFit: 'cover', borderRadius: 'var(--radius-1)' }}
                            />
                          }
                          endContent={
                            seriesMapped ? <Text type="supporting">Open</Text> :
                            manualPending ? <Text type="supporting" color="accent">Match</Text> :
                            isSelected ? <Check size={16} color="var(--color-accent)" /> : undefined
                          }
                        />
                      );
                    })}
                  </List>
                )}
                {selectedSonarr && !mappedSeriesIds.has(selectedSonarr.id) && (
                  <Button label="Continue" variant="primary" onClick={goToMatchStep} />
                )}
              </VStack>
            )
          )}

          {addStep === ADD_STEP_FALLBACK_AFL && (
            <VStack gap={4}>
              <TextInput
                label="Cached AFL title"
                value={fallbackAflQuery}
                onChange={(v) => { setFallbackAflQuery(v); scheduleFallbackAflFilter(); }}
                placeholder="Filter cached AnimeFillerList titles"
              />
              {fallbackAflLoading ? (
                <VStack gap={3} hAlign="center" padding={10}>
                  <Spinner size="lg" label="Loading cached AnimeFillerList titles" />
                </VStack>
              ) : fallbackAflResults.length === 0 ? (
                <EmptyState title="No cached AFL titles found" description="Try another filter or refresh the AFL cache in Settings if this anime is new." />
              ) : (
                <List hasDividers style={{ maxHeight: 'var(--spacing-80)', overflow: 'auto' }}>
                  {fallbackAflResults.map((result) => {
                    const isSelected = selectedFallbackAfl?.slug === result.slug;
                    return (
                      <ListItem
                        key={result.slug}
                        label={result.title || ''}
                        onClick={() => setSelectedFallbackAfl(result)}
                        endContent={isSelected ? <Check size={16} color="var(--color-accent)" /> : undefined}
                      />
                    );
                  })}
                </List>
              )}
              {selectedFallbackAfl && (
                <Button label="Continue to Sonarr lookup" variant="primary" onClick={goToFallbackSonarrStep} />
              )}
            </VStack>
          )}

          {addStep === ADD_STEP_FALLBACK_SONARR && (
            <VStack gap={4}>
              <TextInput
                label="Sonarr lookup"
                value={sonarrLookupQuery}
                onChange={setSonarrLookupQuery}
                placeholder="Search Sonarr for series to add"
                startIcon={Search}
              />
              <Button label="Search Sonarr" variant="secondary" onClick={loadSonarrLookupResults} isLoading={sonarrLookupLoading} />
              {sonarrLookupResults.length > 0 && (
                <List hasDividers style={{ maxHeight: 'var(--spacing-80)', overflow: 'auto' }}>
                  {sonarrLookupResults.map((series) => {
                    const isSelected = selectedSonarrLookup?.id === series.id;
                    return (
                      <ListItem
                        key={series.id}
                        label={series.title}
                        description={String(series.year || 'Anime')}
                        onClick={() => setSelectedSonarrLookup(series)}
                        endContent={isSelected ? <Check size={16} color="var(--color-accent)" /> : undefined}
                      />
                    );
                  })}
                </List>
              )}
              {selectedSonarrLookup && (
                <Button label="Add to Sonarr and match" variant="primary" onClick={handleAddSelectedSonarrLookup} isLoading={sonarrAdding} />
              )}
            </VStack>
          )}

          {(addStep === ADD_STEP_MATCH || addStep === ADD_STEP_UNSAFE_MATCH) && (
            <VStack gap={4}>
              {matchLoading ? (
                <VStack gap={3} hAlign="center" padding={10}>
                  <Spinner size="lg" label="Loading AFL suggestions" />
                </VStack>
              ) : (
                <>
                  {manualSearchVisible && (
                    <VStack gap={2}>
                      <TextInput
                        label="Manual AFL search"
                        value={manualQuery}
                        onChange={setManualQuery}
                        placeholder="Search AnimeFillerList"
                        startIcon={Search}
                      />
                      <Button label="Search" variant="secondary" onClick={handleManualSearch} isLoading={manualSearching} />
                    </VStack>
                  )}
                  {displayedMatches.length > 0 ? (
                    <List hasDividers style={{ maxHeight: 'var(--spacing-80)', overflow: 'auto' }}>
                      {displayedMatches.map((match) => {
                        const isSelected = matchSlug(selectedAfl) === matchSlug(match);
                        const unsafe = isUnsafeMatch(match);
                        return (
                          <ListItem
                            key={matchSlug(match)}
                            label={matchTitle(match)}
                            description={unsafe ? 'Needs confirmation' : undefined}
                            onClick={() => setSelectedAfl(match)}
                            endContent={
                              <HStack gap={2} align="center">
                                {match.confidence != null && (
                                  <Text type="supporting" hasTabularNumbers>{Math.round(match.confidence * 100)}%</Text>
                                )}
                                {isSelected && <Check size={16} color="var(--color-accent)" />}
                              </HStack>
                            }
                          />
                        );
                      })}
                    </List>
                  ) : (
                    !manualSearchVisible && <EmptyState title="No AFL matches" description="Try a manual search above." />
                  )}

                  {/* Tracking rules */}
                  <Divider />
                  <Heading level={3}>Tracking rules</Heading>
                  <Switch
                    label="Track filler"
                    description="Monitor filler episodes in Sonarr"
                    value={addTrackFiller}
                    onChange={setAddTrackFiller}
                  />
                  <Switch
                    label="Track mixed canon"
                    description="Monitor mixed canon/filler episodes"
                    value={addTrackMixed}
                    onChange={setAddTrackMixed}
                  />
                  <Switch
                    label="Track anime canon"
                    description="Monitor anime-original episodes"
                    value={addTrackAnimeCanon}
                    onChange={setAddTrackAnimeCanon}
                  />
                  <Selector
                    label="Start tracking from"
                    options={startModeOptions}
                    value={addStartMode}
                    onChange={setAddStartMode}
                  />

                  {addStep === ADD_STEP_UNSAFE_MATCH ? (
                    <Button label="Confirm match" variant="primary" onClick={handleConfirmMatch} isLoading={confirming} />
                  ) : (
                    <HStack gap={2}>
                      <Button label="Keep Sonarr only" variant="secondary" onClick={handleKeepSonarrOnly} />
                      <Button label="Confirm match" variant="primary" onClick={handlePrimaryMatchAction} isLoading={confirming} />
                    </HStack>
                  )}
                </>
              )}
            </VStack>
          )}

          {addStep === ADD_STEP_POST_ADD && (
            <VStack gap={4} hAlign="center" padding={6}>
              <Check size={32} color="var(--color-success)" />
              <Heading level={3}>{addOutcome?.title}</Heading>
              <Text type="supporting">
                {addOutcome?.type === 'mapping_created'
                  ? 'Now tracked by Mizu.'
                  : 'In Sonarr now, but still needs a safe AFL mapping before Mizu can track it.'}
              </Text>
              <HStack gap={2}>
                {addOutcome?.mappingId && (
                  <Button label="Open anime" variant="primary" onClick={() => { setShowAddModal(false); openShow(addOutcome.mappingId!); }} />
                )}
                <Button label="Add another" variant="secondary" onClick={() => returnToSeriesStep()} />
              </HStack>
            </VStack>
          )}
        </VStack>
      </Dialog>
    </Section>
  );
}
