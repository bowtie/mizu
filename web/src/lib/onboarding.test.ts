import { describe, it, expect, beforeEach } from "vitest";
import { needsOnboarding, skipOnboarding, isOnboardingSkipped } from "./onboarding";

describe("onboarding", () => {
  beforeEach(() => localStorage.clear());

  it("is needed only when Sonarr is known to be unconfigured", () => {
    expect(needsOnboarding(false)).toBe(true);
    expect(needsOnboarding(true)).toBe(false);
    expect(needsOnboarding(undefined)).toBe(false);
  });

  it("stops redirecting once skipped", () => {
    expect(isOnboardingSkipped()).toBe(false);
    skipOnboarding();
    expect(isOnboardingSkipped()).toBe(true);
    expect(needsOnboarding(false)).toBe(false);
  });
});
