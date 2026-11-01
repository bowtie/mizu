/** Library should send a fresh install to /welcome until Sonarr is connected. */
export function needsOnboarding(sonarrKeySet: boolean | undefined): boolean {
  return sonarrKeySet === false;
}
