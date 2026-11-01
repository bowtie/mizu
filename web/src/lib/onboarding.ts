const SKIPPED_KEY = "mizu.onboarding.skipped";

/** True when the user chose "Skip setup" on the welcome page. */
export function isOnboardingSkipped(): boolean {
  try {
    return localStorage.getItem(SKIPPED_KEY) === "1";
  } catch {
    return false;
  }
}

export function skipOnboarding(): void {
  try {
    localStorage.setItem(SKIPPED_KEY, "1");
  } catch {
    /* storage unavailable — the redirect simply stays active */
  }
}

/** Library should send a fresh install to /welcome until Sonarr is connected. */
export function needsOnboarding(sonarrKeySet: boolean | undefined): boolean {
  return sonarrKeySet === false && !isOnboardingSkipped();
}
