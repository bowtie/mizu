import { describe, it, expect } from "vitest";
import { needsOnboarding } from "./onboarding";

describe("onboarding", () => {
  it("is needed only when Sonarr is known to be unconfigured", () => {
    expect(needsOnboarding(false)).toBe(true);
    expect(needsOnboarding(true)).toBe(false);
    expect(needsOnboarding(undefined)).toBe(false);
  });
});
