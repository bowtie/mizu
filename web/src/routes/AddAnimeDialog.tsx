import { useEffect, useRef } from "react";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Check, X } from "lucide-react";
import { HTTPError } from "ky";
import { lookupSonarrSeries, searchAFL, suggestAFL, errorMessage } from "../lib/api";
import { useAddSonarrSeries, useConfirmMatch, useMarkManualMatchPending } from "../lib/queries";
import type { AFLMatch, SonarrAddPayload, SonarrSeries } from "../lib/types";

// ── Constants, types, and helpers are in addWizardHelpers.ts ──
import {
  ADD_STEP_SERIES,
  ADD_STEP_FALLBACK_AFL,
  ADD_STEP_FALLBACK_SONARR,
  ADD_STEP_MATCH,
  ADD_STEP_UNSAFE_MATCH,
  ADD_STEP_POST_ADD,
  trackedToAction,
} from "./addWizardHelpers";
import type { AddWizardState, AddWizardAction } from "./addWizardHelpers";

import {
  isCompactMatchSelection,
  isUnsafeMatch,
  matchSlug,
  shouldAutoCreateMappingAfterSonarrAdd,
} from "../components/addAnime/matchHelpers";
import { SeriesStep } from "../components/addAnime/SeriesStep";
import { FallbackAflStep } from "../components/addAnime/FallbackAflStep";
import { FallbackSonarrStep } from "../components/addAnime/FallbackSonarrStep";
import { MatchStep } from "../components/addAnime/MatchStep";
import { PostAddStep } from "../components/addAnime/PostAddStep";

// ── Props ──

export interface AddAnimeDialogProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  allSeries: SonarrSeries[];
  mappedSeriesIds: Set<string | number>;
  filteredSeriesLibrary: SonarrSeries[];
  activeWizardSteps: { key: string; label: string }[];
  currentStepIndex: number;
  loadLanding: (forceRefresh?: boolean) => Promise<void>;
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

  // These update the cached series lists themselves (see queries.ts).
  const { mutateAsync: confirmMatch } = useConfirmMatch();
  const { mutateAsync: addSonarrSeries } = useAddSonarrSeries();
  const { mutateAsync: markManualMatchPending } = useMarkManualMatchPending();

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
      addNotification("Couldn't load AFL suggestions: " + errorMessage(err), "error");
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

  const handleConfirmMatch = async (overrideAfl?: AFLMatch | null, overwrite = false) => {
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
        ...(overwrite ? { overwrite: true } : {}),
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
      await loadLanding(true);
    } catch (err: unknown) {
      // The backend refuses to silently overwrite an existing mapping
      // (409) so a stale tab can't reset another session's config. Ask
      // the user to confirm replacing it, then retry once.
      if (
        !overwrite &&
        err instanceof HTTPError &&
        err.response.status === 409 &&
        w.selectedSonarr
      ) {
        const replace = window.confirm(
          `"${w.selectedSonarr.title}" already has a mapping in Mizu. ` +
            "Overwrite it with this match? Its tracking rules and progress will be replaced.",
        );
        if (replace) {
          dispatch({ type: "SET", payload: { confirming: false } });
          await handleConfirmMatch(overrideAfl, true);
          return;
        }
        return; // declined — no error toast
      }
      addNotification("Couldn't sync anime: " + errorMessage(err), "error");
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
    } catch (err: unknown) {
      addNotification("Couldn't save the manual match reminder: " + errorMessage(err), "error");
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
      addNotification("Couldn't add anime to Sonarr: " + errorMessage(err), "error");
    } finally {
      dispatch({ type: "SET", payload: { sonarrAdding: false } });
    }
  };

  const returnToSeriesStep = (prefill: string = "") => {
    dispatch({ type: "RETURN_TO_SERIES_STEP", prefill });
  };

  const stepDescription = (): string => {
    if (w.addStep === ADD_STEP_SERIES)
      return "Pick an anime from your Sonarr library, or add a new one from the AFL list.";
    if (w.addStep === ADD_STEP_FALLBACK_AFL)
      return "Pick the AnimeFillerList title, then Mizu looks it up in Sonarr.";
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
          data-cuelume-press
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
              <span className="wizard-step-dot">{isPast ? <Check size={12} /> : i + 1}</span>
              <span className="wizard-step-label">{step.label}</span>
            </li>
          );
        })}
      </ol>

      {w.addStep !== ADD_STEP_POST_ADD && (
        <Text type="supporting" color="secondary" style={{ textAlign: "center" }}>
          {stepDescription()}
        </Text>
      )}

      {/* Step content — keyed div re-mounts on step change for fade-in animation */}
      <div key={w.addStep} className="wizard-step-content">
        {w.addStep === ADD_STEP_SERIES && (
          <SeriesStep
            w={w}
            dispatch={dispatch}
            allSeries={allSeries}
            filteredSeriesLibrary={filteredSeriesLibrary}
            mappedSeriesIds={mappedSeriesIds}
            posterUrl={posterUrl}
            openExistingMapping={openExistingMapping}
            goToFallbackAflStep={goToFallbackAflStep}
            goToMatchStep={goToMatchStep}
          />
        )}

        {w.addStep === ADD_STEP_FALLBACK_AFL && (
          <FallbackAflStep
            w={w}
            dispatch={dispatch}
            scheduleFallbackAflFilter={scheduleFallbackAflFilter}
            goToFallbackSonarrStep={goToFallbackSonarrStep}
          />
        )}

        {w.addStep === ADD_STEP_FALLBACK_SONARR && (
          <FallbackSonarrStep
            w={w}
            dispatch={dispatch}
            loadSonarrLookupResults={loadSonarrLookupResults}
            handleAddSelectedSonarrLookup={handleAddSelectedSonarrLookup}
          />
        )}

        {(w.addStep === ADD_STEP_MATCH || w.addStep === ADD_STEP_UNSAFE_MATCH) && (
          <MatchStep
            w={w}
            dispatch={dispatch}
            handleManualSearch={handleManualSearch}
            handleConfirmMatch={handleConfirmMatch}
            handlePrimaryMatchAction={handlePrimaryMatchAction}
            handleKeepSonarrOnly={handleKeepSonarrOnly}
          />
        )}

        {w.addStep === ADD_STEP_POST_ADD && (
          <PostAddStep
            w={w}
            dispatch={dispatch}
            openShow={openShow}
            returnToSeriesStep={returnToSeriesStep}
          />
        )}
      </div>
    </VStack>
  );
}
