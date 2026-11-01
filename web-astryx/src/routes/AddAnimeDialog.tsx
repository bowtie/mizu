import { useEffect, useRef } from "react";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Spinner } from "@astryxdesign/core/Spinner";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Selector } from "@astryxdesign/core/Selector";
import { Switch } from "@astryxdesign/core/Switch";
import { List, ListItem } from "@astryxdesign/core/List";
import { Divider } from "@astryxdesign/core/Divider";
import { Search, Check, X, CircleAlert } from "lucide-react";
import {
  addSonarrSeries,
  confirmMatch,
  lookupSonarrSeries,
  markManualMatchPending,
  searchAFL,
  suggestAFL,
  errorMessage,
} from "../lib/api";
import type { AFLMatch, SelectOption, SonarrAddPayload, SonarrSeries } from "../lib/types";

// ── Constants ──

const START_FROM_BEGINNING = "from_beginning";
export const START_FROM_CURRENT = "from_current_progress";
export const ADD_STEP_SERIES = "series";
export const ADD_STEP_FALLBACK_AFL = "fallback_afl";
export const ADD_STEP_FALLBACK_SONARR = "fallback_sonarr";
export const ADD_STEP_MATCH = "match";
export const ADD_STEP_UNSAFE_MATCH = "unsafe_match";
export const ADD_STEP_POST_ADD = "post_add";

const startModeOptions: SelectOption[] = [
  { value: START_FROM_CURRENT, label: "Current progress" },
  { value: START_FROM_BEGINNING, label: "Episode 1" },
];

const wizardSteps = [
  { key: ADD_STEP_SERIES, label: "Select" },
  { key: ADD_STEP_MATCH, label: "Match" },
  { key: ADD_STEP_UNSAFE_MATCH, label: "Confirm" },
  { key: ADD_STEP_POST_ADD, label: "Done" },
];
const wizardFallbackSteps = [
  { key: ADD_STEP_FALLBACK_AFL, label: "AFL Search" },
  { key: ADD_STEP_FALLBACK_SONARR, label: "Add to Sonarr" },
  { key: ADD_STEP_MATCH, label: "Match" },
];

export function getWizardSteps(addStep: string) {
  return [ADD_STEP_FALLBACK_AFL, ADD_STEP_FALLBACK_SONARR].includes(addStep)
    ? wizardFallbackSteps
    : wizardSteps;
}

// ── Types ──

interface AddOutcome {
  title: string;
  type: string;
  mappingId?: string | number | null;
}

export interface AddWizardState {
  showAddModal: boolean;
  addStep: string;
  sonarrLoading: boolean;
  seriesQuery: string;
  selectedSonarr: SonarrSeries | null;
  displayedMatches: AFLMatch[];
  matchLoading: boolean;
  manualQuery: string;
  manualSearching: boolean;
  manualSearchVisible: boolean;
  selectedAfl: AFLMatch | null;
  confirming: boolean;
  addTrackFiller: boolean;
  addTrackMixed: boolean;
  addTrackAnimeCanon: boolean;
  addStartMode: string;
  fallbackAflQuery: string;
  cachedAflShows: AFLMatch[];
  fallbackAflResults: AFLMatch[];
  fallbackAflLoading: boolean;
  selectedFallbackAfl: AFLMatch | null;
  sonarrLookupQuery: string;
  sonarrLookupResults: SonarrSeries[];
  sonarrLookupLoading: boolean;
  selectedSonarrLookup: SonarrSeries | null;
  sonarrAdding: boolean;
  sonarrRootFolderOptions: SelectOption[];
  sonarrQualityProfileOptions: SelectOption[];
  fallbackQualityProfileId: string;
  fallbackSeasonFolder: boolean;
  addOutcome: AddOutcome | null;
}

export type AddWizardAction =
  | { type: "SET"; payload: Partial<AddWizardState> }
  | { type: "RESET_FALLBACK_FLOW" }
  | { type: "RETURN_TO_SERIES_STEP"; prefill?: string }
  | { type: "OPEN_ADD_MODAL_INIT" };

export const INITIAL_WIZARD: AddWizardState = {
  showAddModal: false,
  addStep: ADD_STEP_SERIES,
  sonarrLoading: false,
  seriesQuery: "",
  selectedSonarr: null,
  displayedMatches: [],
  matchLoading: false,
  manualQuery: "",
  manualSearching: false,
  manualSearchVisible: false,
  selectedAfl: null,
  confirming: false,
  addTrackFiller: false,
  addTrackMixed: true,
  addTrackAnimeCanon: true,
  addStartMode: START_FROM_CURRENT,
  fallbackAflQuery: "",
  cachedAflShows: [],
  fallbackAflResults: [],
  fallbackAflLoading: false,
  selectedFallbackAfl: null,
  sonarrLookupQuery: "",
  sonarrLookupResults: [],
  sonarrLookupLoading: false,
  selectedSonarrLookup: null,
  sonarrAdding: false,
  sonarrRootFolderOptions: [],
  sonarrQualityProfileOptions: [],
  fallbackQualityProfileId: "",
  fallbackSeasonFolder: true,
  addOutcome: null,
};

export function addWizardReducer(state: AddWizardState, action: AddWizardAction): AddWizardState {
  switch (action.type) {
    case "SET":
      return { ...state, ...action.payload };
    case "RESET_FALLBACK_FLOW":
      return {
        ...state,
        fallbackAflQuery: "",
        fallbackAflResults: state.cachedAflShows,
        selectedFallbackAfl: null,
        sonarrLookupQuery: "",
        sonarrLookupResults: [],
        selectedSonarrLookup: null,
        fallbackQualityProfileId: "",
        fallbackSeasonFolder: true,
        addOutcome: null,
        selectedAfl: null,
      };
    case "RETURN_TO_SERIES_STEP":
      return {
        ...state,
        addStep: ADD_STEP_SERIES,
        addOutcome: null,
        selectedAfl: null,
        selectedFallbackAfl: null,
        selectedSonarrLookup: null,
        sonarrLookupResults: [],
        fallbackAflResults: [],
        seriesQuery: action.prefill ?? state.seriesQuery,
      };
    case "OPEN_ADD_MODAL_INIT":
      return {
        ...state,
        showAddModal: true,
        addStep: ADD_STEP_SERIES,
        seriesQuery: "",
        selectedSonarr: null,
        displayedMatches: [],
        selectedAfl: null,
        manualQuery: "",
        manualSearchVisible: false,
        fallbackAflQuery: "",
        fallbackAflResults: state.cachedAflShows,
        selectedFallbackAfl: null,
        sonarrLookupQuery: "",
        sonarrLookupResults: [],
        selectedSonarrLookup: null,
        fallbackQualityProfileId: "",
        fallbackSeasonFolder: true,
        addOutcome: null,
      };
    default:
      return state;
  }
}

// ── Helpers ──

export const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean =>
  (action || (fallback ? "monitor" : "unmonitor")) === "monitor";
export const trackedToAction = (tracked: boolean): string => (tracked ? "monitor" : "unmonitor");

function isUnsafeMatch(match: AFLMatch | null): boolean {
  return !!match && match.confidence == null;
}
function isCompactMatchSelection(match: AFLMatch | null): boolean {
  if (!match || isUnsafeMatch(match)) return false;
  return (
    match.method === "exact" || match.method === "normalized" || (match.confidence ?? 0) >= 0.95
  );
}
function shouldAutoCreateMappingAfterSonarrAdd(match: AFLMatch | null): boolean {
  return (
    !!match && !isUnsafeMatch(match) && match.method === "exact" && (match.confidence ?? 0) >= 1
  );
}
function matchTitle(match: AFLMatch | null | undefined): string {
  return match?.afl_show?.title || match?.title || "";
}
function matchSlug(match: AFLMatch | null | undefined): string {
  return match?.afl_show?.slug || match?.slug || "";
}

// ── Props ──

export interface AddAnimeDialogProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  allSeries: SonarrSeries[];
  setAllSeries: React.Dispatch<React.SetStateAction<SonarrSeries[]>>;
  setSeriesById: React.Dispatch<React.SetStateAction<Record<string | number, SonarrSeries>>>;
  mappedSeriesIds: Set<string | number>;
  filteredSeriesLibrary: SonarrSeries[];
  activeWizardSteps: { key: string; label: string }[];
  currentStepIndex: number;
  loadLanding: () => Promise<void>;
  openShow: (id: string | number) => void;
  openExistingMapping: (seriesId: string | number) => void;
  addNotification: (message: string, type?: string) => void;
  preferredAnimeRootFolder: string;
  preferredAnimeQualityProfileId: string;
  posterUrl: (series: SonarrSeries | null | undefined) => string;
}

// ── Component ──

export function AddAnimeDialog(props: AddAnimeDialogProps) {
  const {
    w,
    dispatch,
    allSeries,
    setAllSeries,
    setSeriesById,
    mappedSeriesIds,
    filteredSeriesLibrary,
    activeWizardSteps,
    currentStepIndex,
    loadLanding,
    openShow,
    openExistingMapping,
    addNotification,
    preferredAnimeRootFolder,
    preferredAnimeQualityProfileId,
    posterUrl,
  } = props;

  const fallbackFilterTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const suggestReqId = useRef(0);
  const lookupReqId = useRef(0);

  useEffect(() => {
    return () => {
      if (fallbackFilterTimeout.current) clearTimeout(fallbackFilterTimeout.current);
    };
  }, []);

  // ── Wizard handlers ──

  const loadSuggestedMatches = async () => {
    if (!w.selectedSonarr) return;
    const reqId = ++suggestReqId.current;
    dispatch({ type: "SET", payload: { matchLoading: true, selectedAfl: null } });
    try {
      const suggested = await suggestAFL(w.selectedSonarr.id);
      if (reqId !== suggestReqId.current) return; // stale response
      const visible = suggested.length === 0 || !!w.selectedSonarr.manualMatchPending;
      dispatch({
        type: "SET",
        payload: { displayedMatches: suggested, manualSearchVisible: visible },
      });
      const bestMatch = suggested[0];
      if (bestMatch && isCompactMatchSelection(bestMatch)) {
        dispatch({ type: "SET", payload: { selectedAfl: bestMatch } });
      }
      if (visible && w.selectedSonarr.manualMatchPending && w.manualQuery.trim()) {
        const manualResults = await searchAFL(w.manualQuery.trim(), w.selectedSonarr.id);
        if (reqId !== suggestReqId.current) return; // stale response
        dispatch({
          type: "SET",
          payload: { displayedMatches: manualResults },
        });
      }
    } catch (err: unknown) {
      if (reqId !== suggestReqId.current) return;
      dispatch({ type: "SET", payload: { displayedMatches: [], manualSearchVisible: true } });
      addNotification("Failed to load AFL suggestions: " + errorMessage(err), "error");
    }
    if (reqId === suggestReqId.current) {
      dispatch({ type: "SET", payload: { matchLoading: false } });
    }
  };

  const goToMatchStep = async () => {
    if (!w.selectedSonarr) return;
    dispatch({
      type: "SET",
      payload: { addStep: ADD_STEP_MATCH, manualQuery: w.selectedSonarr.title || "" },
    });
    await loadSuggestedMatches();
  };

  const handleManualSearch = async () => {
    if (!w.selectedSonarr || !w.manualSearchVisible) return;
    const reqId = ++suggestReqId.current;
    dispatch({ type: "SET", payload: { manualSearching: true, selectedAfl: null } });
    try {
      const results = await searchAFL(w.manualQuery.trim(), w.selectedSonarr.id);
      if (reqId !== suggestReqId.current) return;
      dispatch({ type: "SET", payload: { displayedMatches: results } });
    } catch (err: unknown) {
      if (reqId !== suggestReqId.current) return;
      dispatch({ type: "SET", payload: { displayedMatches: [] } });
      addNotification("AFL search failed: " + errorMessage(err), "error");
    }
    if (reqId === suggestReqId.current) {
      dispatch({ type: "SET", payload: { manualSearching: false } });
    }
  };

  const handleConfirmMatch = async (overrideAfl?: AFLMatch | null) => {
    const afl = overrideAfl ?? w.selectedAfl;
    if (!w.selectedSonarr || !afl) return;
    dispatch({ type: "SET", payload: { confirming: true } });
    try {
      const created = await confirmMatch({
        sonarr_series_id: w.selectedSonarr.id,
        sonarr_title: w.selectedSonarr.title,
        afl_slug: afl.afl_show?.slug || afl.slug,
        afl_title: afl.afl_show?.title || afl.title,
        filler_action: trackedToAction(w.addTrackFiller),
        mixed_action: trackedToAction(w.addTrackMixed),
        anime_canon_action: trackedToAction(w.addTrackAnimeCanon),
        start_mode: w.addStartMode,
      });
      addNotification(`Added "${w.selectedSonarr.title}" to Mizu`, "success");
      dispatch({
        type: "SET",
        payload: {
          addOutcome: {
            title: w.selectedSonarr.title,
            type: "mapping_created",
            mappingId: created?.id || null,
          },
          addStep: ADD_STEP_POST_ADD,
        },
      });
      await loadLanding();
    } catch (err: unknown) {
      addNotification("Failed to sync anime: " + errorMessage(err), "error");
    } finally {
      dispatch({ type: "SET", payload: { confirming: false } });
    }
  };

  const handlePrimaryMatchAction = async () => {
    if (!w.selectedAfl) return;
    if (isUnsafeMatch(w.selectedAfl)) {
      dispatch({ type: "SET", payload: { addStep: ADD_STEP_UNSAFE_MATCH } });
      return;
    }
    await handleConfirmMatch();
  };

  const handleKeepSonarrOnly = async () => {
    if (!w.selectedSonarr?.id) return;
    try {
      await markManualMatchPending(w.selectedSonarr.id);
      dispatch({
        type: "SET",
        payload: {
          addOutcome: { title: w.selectedSonarr.title, type: "pending_manual_match" },
          addStep: ADD_STEP_POST_ADD,
        },
      });
      setAllSeries((prev) =>
        prev.map((s) => (s.id === w.selectedSonarr?.id ? { ...s, manualMatchPending: true } : s)),
      );
    } catch (err: unknown) {
      addNotification("Failed to persist manual match reminder: " + errorMessage(err), "error");
    }
  };

  // ── Fallback flow ──

  const ensureCachedAflShowsLoaded = async () => {
    if (w.cachedAflShows.length > 0) {
      dispatch({ type: "SET", payload: { fallbackAflResults: w.cachedAflShows } });
      return;
    }
    dispatch({ type: "SET", payload: { fallbackAflLoading: true, selectedFallbackAfl: null } });
    try {
      const cached = await searchAFL("");
      dispatch({ type: "SET", payload: { cachedAflShows: cached, fallbackAflResults: cached } });
    } catch (err: unknown) {
      dispatch({ type: "SET", payload: { cachedAflShows: [], fallbackAflResults: [] } });
      addNotification("AFL cache load failed: " + errorMessage(err), "error");
    } finally {
      dispatch({ type: "SET", payload: { fallbackAflLoading: false } });
    }
  };

  const filterFallbackAflResults = (query: string = w.fallbackAflQuery) => {
    const normalized = (query || "").trim().toLowerCase();
    if (!normalized) {
      dispatch({ type: "SET", payload: { fallbackAflResults: w.cachedAflShows } });
      return;
    }
    const tokens = normalized.split(/\s+/).filter(Boolean);
    dispatch({
      type: "SET",
      payload: {
        fallbackAflResults: w.cachedAflShows.filter((show) => {
          const title = (show.title || "").toLowerCase();
          if (!title) return false;
          if (title.includes(normalized)) return true;
          return tokens.every((token) => title.includes(token));
        }),
      },
    });
  };

  const scheduleFallbackAflFilter = () => {
    if (fallbackFilterTimeout.current) clearTimeout(fallbackFilterTimeout.current);
    fallbackFilterTimeout.current = setTimeout(() => filterFallbackAflResults(), 120);
  };

  const goToFallbackAflStep = async (prefill: string = "") => {
    dispatch({ type: "RESET_FALLBACK_FLOW" });
    dispatch({
      type: "SET",
      payload: {
        addStep: ADD_STEP_FALLBACK_AFL,
        fallbackAflQuery: prefill,
        fallbackQualityProfileId: preferredAnimeQualityProfileId,
      },
    });
    await ensureCachedAflShowsLoaded();
    filterFallbackAflResults(prefill);
  };

  const loadSonarrLookupResults = async (query?: string) => {
    const q = (query ?? w.sonarrLookupQuery).trim();
    if (!q) return;
    const reqId = ++lookupReqId.current;
    dispatch({ type: "SET", payload: { sonarrLookupLoading: true, selectedSonarrLookup: null } });
    try {
      const results = await lookupSonarrSeries(q);
      if (reqId !== lookupReqId.current) return;
      dispatch({ type: "SET", payload: { sonarrLookupResults: results } });
    } catch (err: unknown) {
      if (reqId !== lookupReqId.current) return;
      dispatch({ type: "SET", payload: { sonarrLookupResults: [] } });
      addNotification("Sonarr lookup failed: " + errorMessage(err), "error");
    }
    if (reqId === lookupReqId.current) {
      dispatch({ type: "SET", payload: { sonarrLookupLoading: false } });
    }
  };

  const goToFallbackSonarrStep = async () => {
    if (!w.selectedFallbackAfl) return;
    const query = w.selectedFallbackAfl.title || "";
    dispatch({
      type: "SET",
      payload: {
        addStep: ADD_STEP_FALLBACK_SONARR,
        sonarrLookupQuery: query,
      },
    });
    await loadSonarrLookupResults(query);
  };

  const buildSonarrAddPayload = (series: SonarrSeries): SonarrAddPayload => ({
    series: { ...series, seriesType: "anime" },
    root_folder_path: preferredAnimeRootFolder,
    quality_profile_id: Number(w.fallbackQualityProfileId || 0),
    monitor_mode: "none",
    season_folder: w.fallbackSeasonFolder,
    search_on_add: false,
  });

  const handleAddSelectedSonarrLookup = async () => {
    if (!w.selectedFallbackAfl || !w.selectedSonarrLookup) return;
    const fallbackAflSlug = w.selectedFallbackAfl.slug;
    dispatch({ type: "SET", payload: { sonarrAdding: true } });
    try {
      const created = await addSonarrSeries(buildSonarrAddPayload(w.selectedSonarrLookup));
      const nextAll = [...allSeries.filter((s) => s.id !== created.id), created].sort((a, b) =>
        a.title.localeCompare(b.title),
      );
      setAllSeries(nextAll);
      setSeriesById(Object.fromEntries(nextAll.map((s) => [s.id, s])));
      dispatch({
        type: "SET",
        payload: {
          selectedSonarr: created,
          manualQuery: w.selectedFallbackAfl.title || created.title || "",
        },
      });

      const query = w.selectedFallbackAfl.title || created.title || "";
      const results = await searchAFL(query.trim(), created.id);
      const visible = results.length === 0 || !!created.manualMatchPending;
      dispatch({
        type: "SET",
        payload: { displayedMatches: results, manualSearchVisible: visible },
      });
      const chosenResult = results.find((item) => matchSlug(item) === fallbackAflSlug);
      if (chosenResult && shouldAutoCreateMappingAfterSonarrAdd(chosenResult)) {
        dispatch({ type: "SET", payload: { selectedAfl: chosenResult } });
        await handleConfirmMatch(chosenResult);
        return;
      }
      if (chosenResult && !isUnsafeMatch(chosenResult)) {
        dispatch({ type: "SET", payload: { selectedAfl: chosenResult, addStep: ADD_STEP_MATCH } });
        return;
      }
      if (chosenResult) {
        dispatch({
          type: "SET",
          payload: { selectedAfl: chosenResult, addStep: ADD_STEP_UNSAFE_MATCH },
        });
        return;
      }
      await handleKeepSonarrOnly();
    } catch (err: unknown) {
      addNotification("Failed to add anime to Sonarr: " + errorMessage(err), "error");
    } finally {
      dispatch({ type: "SET", payload: { sonarrAdding: false } });
    }
  };

  const returnToSeriesStep = (prefill: string = "") => {
    dispatch({ type: "RETURN_TO_SERIES_STEP", prefill });
  };

  const stepDescription = (): string => {
    if (w.addStep === ADD_STEP_SERIES)
      return "Search your actionable Sonarr anime library first. If the anime is missing, switch to cached AFL search and add it into Sonarr from here.";
    if (w.addStep === ADD_STEP_FALLBACK_AFL)
      return "Search cached AnimeFillerList titles, then use the chosen title to look up the right Sonarr series before adding it.";
    if (w.addStep === ADD_STEP_FALLBACK_SONARR)
      return `Confirm the Sonarr series Mizu should add for ${w.selectedFallbackAfl?.title || ""}.`;
    if (w.addStep === ADD_STEP_POST_ADD) {
      if (w.addOutcome?.type === "mapping_created")
        return `${w.addOutcome?.title} is now tracked by Mizu.`;
      return `${w.addOutcome?.title} is in Sonarr now, but still needs a safe AFL mapping before Mizu can track it.`;
    }
    if (w.addStep === ADD_STEP_UNSAFE_MATCH)
      return `Mizu found a possible cached AFL entry for ${w.selectedSonarr?.title || ""}, but it needs your confirmation before creating the mapping.`;
    return `Match ${w.selectedSonarr?.title || ""} with the correct AnimeFillerList entry.`;
  };

  // ── Render ──

  return (
    <VStack gap={4} padding={6} isScrollable height="100%">
      <HStack justify="between" align="center">
        <Heading level={2}>Add anime</Heading>
        <Button
          label="Close"
          variant="ghost"
          isIconOnly
          icon={<X size={16} />}
          onClick={() => dispatch({ type: "SET", payload: { showAddModal: false } })}
        />
      </HStack>

      {/* Step progress */}
      <ol className="wizard-steps" aria-label="Wizard progress">
        {activeWizardSteps.map((step, i) => {
          const isCurrent = step.key === w.addStep;
          const isPast = currentStepIndex > i;
          return (
            <li
              key={step.key}
              className="wizard-step"
              data-state={isCurrent ? "current" : isPast ? "past" : "future"}
              aria-current={isCurrent ? "step" : undefined}
            >
              <span className="wizard-step-dot">
                {isPast ? <Check size={12} /> : i + 1}
              </span>
              <span className="wizard-step-label">{step.label}</span>
            </li>
          );
        })}
      </ol>

      <Text type="supporting">{stepDescription()}</Text>

      <Divider />

      {/* Step content */}
      {w.addStep === ADD_STEP_SERIES &&
        (w.sonarrLoading ? (
          <VStack gap={3} hAlign="center" padding={6}>
            <Spinner size="lg" label="Loading available Sonarr anime" />
          </VStack>
        ) : allSeries.length === 0 ? (
          <EmptyState
            title="No anime found in Sonarr"
            description="Sonarr returned no anime series for this library."
            icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
          />
        ) : (
          <VStack gap={4}>
            <TextInput
              label="Search"
              value={w.seriesQuery}
              onChange={(v) => dispatch({ type: "SET", payload: { seriesQuery: v } })}
              placeholder="Search Sonarr anime"
              startIcon={Search}
            />
            {filteredSeriesLibrary.length === 0 ? (
              <EmptyState
                title="No anime match that search"
                description="Try a different title, year, or jump into the cached AFL list and add it from there."
                icon={<Icon icon={Search} size="lg" color="secondary" />}
                actions={
                  <Button
                    label="Not in Sonarr? Search AFL and add to Sonarr"
                    variant="secondary"
                    icon={<Search size={16} />}
                    onClick={() => goToFallbackAflStep(w.seriesQuery.trim())}
                  />
                }
              />
            ) : (
              <List hasDividers className="wizard-list">
                {filteredSeriesLibrary.map((series) => {
                  const seriesMapped = mappedSeriesIds.has(series.id);
                  const manualPending = !!series.manualMatchPending;
                  const isSelected = w.selectedSonarr?.id === series.id;
                  return (
                    <ListItem
                      key={series.id}
                      label={series.title}
                      description={`${series.year || "Anime"}${series.statistics?.episodeCount ? ` · ${series.statistics.episodeCount} eps` : ""}${manualPending ? " · Needs match" : seriesMapped ? " · Already added" : ""}`}
                      onClick={() => {
                        if (seriesMapped) {
                          openExistingMapping(series.id);
                          return;
                        }
                        dispatch({ type: "SET", payload: { selectedSonarr: series } });
                      }}
                      startContent={
                        posterUrl(series) ? (
                          <img
                            src={posterUrl(series)}
                            alt=""
                            loading="lazy"
                            className="wizard-poster-thumb"
                          />
                        ) : (
                          <div className="wizard-poster-placeholder" />
                        )
                      }
                      endContent={
                        seriesMapped ? (
                          <Text type="supporting">Open</Text>
                        ) : manualPending ? (
                          <Text type="supporting" color="accent">
                            Match
                          </Text>
                        ) : isSelected ? (
                          <Check size={16} color="var(--color-accent)" />
                        ) : undefined
                      }
                    />
                  );
                })}
              </List>
            )}
            {w.selectedSonarr && !mappedSeriesIds.has(w.selectedSonarr.id) && (
              <Button label="Continue" variant="primary" onClick={goToMatchStep} />
            )}
          </VStack>
        ))}

      {w.addStep === ADD_STEP_FALLBACK_AFL && (
        <VStack gap={4}>
          <TextInput
            label="Cached AFL title"
            value={w.fallbackAflQuery}
            onChange={(v) => {
              dispatch({ type: "SET", payload: { fallbackAflQuery: v } });
              scheduleFallbackAflFilter();
            }}
            placeholder="Filter cached AnimeFillerList titles"
          />
          {w.fallbackAflLoading ? (
            <VStack gap={3} hAlign="center" padding={6}>
              <Spinner size="lg" label="Loading cached AnimeFillerList titles" />
            </VStack>
          ) : w.fallbackAflResults.length === 0 ? (
            <EmptyState
              title="No cached AFL titles found"
              description="Try another filter or refresh the AFL cache in Settings if this anime is new."
              icon={<Icon icon={Search} size="lg" color="secondary" />}
            />
          ) : (
            <List hasDividers className="wizard-list">
              {w.fallbackAflResults.map((result) => {
                const isSelected = w.selectedFallbackAfl?.slug === result.slug;
                return (
                  <ListItem
                    key={result.slug}
                    label={result.title || ""}
                    onClick={() =>
                      dispatch({ type: "SET", payload: { selectedFallbackAfl: result } })
                    }
                    endContent={
                      isSelected ? <Check size={16} color="var(--color-accent)" /> : undefined
                    }
                  />
                );
              })}
            </List>
          )}
          {w.selectedFallbackAfl && (
            <Button
              label="Continue to Sonarr lookup"
              variant="primary"
              onClick={goToFallbackSonarrStep}
            />
          )}
        </VStack>
      )}

      {w.addStep === ADD_STEP_FALLBACK_SONARR && (
        <VStack gap={4}>
          <TextInput
            label="Sonarr lookup"
            value={w.sonarrLookupQuery}
            onChange={(v) => dispatch({ type: "SET", payload: { sonarrLookupQuery: v } })}
            placeholder="Search Sonarr for series to add"
            startIcon={Search}
          />
          <Button
            label="Search Sonarr"
            variant="secondary"
            onClick={() => loadSonarrLookupResults()}
            isLoading={w.sonarrLookupLoading}
          />
          {w.sonarrLookupResults.length > 0 && (
            <List hasDividers className="wizard-list">
              {w.sonarrLookupResults.map((series) => {
                const isSelected = w.selectedSonarrLookup?.id === series.id;
                return (
                  <ListItem
                    key={series.id}
                    label={series.title}
                    description={String(series.year || "Anime")}
                    onClick={() =>
                      dispatch({ type: "SET", payload: { selectedSonarrLookup: series } })
                    }
                    endContent={
                      isSelected ? <Check size={16} color="var(--color-accent)" /> : undefined
                    }
                  />
                );
              })}
            </List>
          )}
          {w.selectedSonarrLookup && (
            <Button
              label="Add to Sonarr and match"
              variant="primary"
              onClick={handleAddSelectedSonarrLookup}
              isLoading={w.sonarrAdding}
            />
          )}
        </VStack>
      )}

      {(w.addStep === ADD_STEP_MATCH || w.addStep === ADD_STEP_UNSAFE_MATCH) && (
        <VStack gap={4}>
          {w.matchLoading ? (
            <VStack gap={3} hAlign="center" padding={6}>
              <Spinner size="lg" label="Loading AFL suggestions" />
            </VStack>
          ) : (
            <>
              {w.manualSearchVisible && (
                <VStack gap={2}>
                  <TextInput
                    label="Manual AFL search"
                    value={w.manualQuery}
                    onChange={(v) => dispatch({ type: "SET", payload: { manualQuery: v } })}
                    placeholder="Search AnimeFillerList"
                    startIcon={Search}
                  />
                  <Button
                    label="Search"
                    variant="secondary"
                    onClick={handleManualSearch}
                    isLoading={w.manualSearching}
                  />
                </VStack>
              )}
              {w.displayedMatches.length > 0 ? (
                <List hasDividers className="wizard-list">
                  {w.displayedMatches.map((match) => {
                    const isSelected = matchSlug(w.selectedAfl) === matchSlug(match);
                    const unsafe = isUnsafeMatch(match);
                    return (
                      <ListItem
                        key={matchSlug(match)}
                        label={matchTitle(match)}
                        description={unsafe ? "Needs confirmation" : undefined}
                        onClick={() => dispatch({ type: "SET", payload: { selectedAfl: match } })}
                        endContent={
                          <HStack gap={2} align="center">
                            {match.confidence != null && (
                              <Text type="supporting" hasTabularNumbers>
                                {Math.round(match.confidence * 100)}%
                              </Text>
                            )}
                            {isSelected && <Check size={16} color="var(--color-accent)" />}
                          </HStack>
                        }
                      />
                    );
                  })}
                </List>
              ) : (
                !w.manualSearchVisible && (
                  <EmptyState
                    title="No AFL matches"
                    description="Try a manual search above."
                    icon={<Icon icon={Search} size="lg" color="secondary" />}
                  />
                )
              )}

              {/* Tracking rules */}
              <Divider />
              <Heading level={3}>Tracking rules</Heading>
              <Switch
                label="Track filler"
                description="Monitor filler episodes in Sonarr"
                value={w.addTrackFiller}
                onChange={(v) => dispatch({ type: "SET", payload: { addTrackFiller: v } })}
              />
              <Switch
                label="Track mixed canon"
                description="Monitor mixed canon/filler episodes"
                value={w.addTrackMixed}
                onChange={(v) => dispatch({ type: "SET", payload: { addTrackMixed: v } })}
              />
              <Switch
                label="Track anime canon"
                description="Monitor anime-original episodes"
                value={w.addTrackAnimeCanon}
                onChange={(v) => dispatch({ type: "SET", payload: { addTrackAnimeCanon: v } })}
              />
              <Selector
                label="Start tracking from"
                options={startModeOptions}
                value={w.addStartMode}
                onChange={(v) => dispatch({ type: "SET", payload: { addStartMode: v } })}
              />

              {w.addStep === ADD_STEP_UNSAFE_MATCH ? (
                <Button
                  label="Confirm match"
                  variant="primary"
                  onClick={handleConfirmMatch}
                  isLoading={w.confirming}
                />
              ) : (
                <HStack gap={2}>
                  <Button
                    label="Keep Sonarr only"
                    variant="secondary"
                    onClick={handleKeepSonarrOnly}
                  />
                  <Button
                    label="Confirm match"
                    variant="primary"
                    onClick={handlePrimaryMatchAction}
                    isLoading={w.confirming}
                  />
                </HStack>
              )}
            </>
          )}
        </VStack>
      )}

      {w.addStep === ADD_STEP_POST_ADD && (
        <VStack gap={4} hAlign="center" padding={6}>
          <Check size={32} color="var(--color-success)" />
          <Heading level={3}>{w.addOutcome?.title}</Heading>
          <Text type="supporting">
            {w.addOutcome?.type === "mapping_created"
              ? "Now tracked by Mizu."
              : "In Sonarr now, but still needs a safe AFL mapping before Mizu can track it."}
          </Text>
          <HStack gap={2}>
            {w.addOutcome?.mappingId && (
              <Button
                label="Open anime"
                variant="primary"
                onClick={() => {
                  dispatch({ type: "SET", payload: { showAddModal: false } });
                  openShow(w.addOutcome!.mappingId!);
                }}
              />
            )}
            <Button label="Add another" variant="secondary" onClick={() => returnToSeriesStep()} />
          </HStack>
        </VStack>
      )}
    </VStack>
  );
}
