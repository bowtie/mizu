import { describe, it, expect } from "vitest";
import {
  ADD_STEP_FALLBACK_AFL,
  ADD_STEP_MATCH,
  ADD_STEP_SERIES,
  ADD_STEP_UNSAFE_MATCH,
  getWizardSteps,
} from "./addWizardHelpers";

const labels = (steps: { label: string }[]) => steps.map((s) => s.label);

describe("getWizardSteps", () => {
  it("skips the Confirm step for matches that don't need one", () => {
    expect(labels(getWizardSteps(ADD_STEP_SERIES))).toEqual(["Select", "Match", "Done"]);
    expect(labels(getWizardSteps(ADD_STEP_MATCH))).toEqual(["Select", "Match", "Done"]);
  });

  it("shows Confirm when the selected match needs it", () => {
    expect(labels(getWizardSteps(ADD_STEP_MATCH, true))).toEqual([
      "Select",
      "Match",
      "Confirm",
      "Done",
    ]);
  });

  it("always shows Confirm while on the confirmation step", () => {
    expect(labels(getWizardSteps(ADD_STEP_UNSAFE_MATCH))).toContain("Confirm");
  });

  it("uses the fallback flow regardless of the confirm flag", () => {
    expect(labels(getWizardSteps(ADD_STEP_FALLBACK_AFL, true))).toEqual([
      "AFL Search",
      "Add to Sonarr",
      "Match",
    ]);
  });
});
