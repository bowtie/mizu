// Wizard helpers extracted from AddAnimeDialog so the dialog component
// can be truly lazy-loaded. These are tiny pure functions/types used by
// ShowsPage for wizard state management — they must stay eager.

import type { AFLMatch, SelectOption, SonarrSeries } from "../lib/types";

// ── Constants ──

export const START_FROM_BEGINNING = "from_beginning";
export const START_FROM_CURRENT = "from_current_progress";
export const ADD_STEP_SERIES = "series";
export const ADD_STEP_FALLBACK_AFL = "fallback_afl";
export const ADD_STEP_FALLBACK_SONARR = "fallback_sonarr";
export const ADD_STEP_MATCH = "match";
export const ADD_STEP_UNSAFE_MATCH = "unsafe_match";
export const ADD_STEP_POST_ADD = "post_add";

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
        selectedSonarr: null,
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
