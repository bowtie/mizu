// Lazy-load cuelume — none of these functions are called on first
// paint. initSounds() runs on idle, applySoundsEnabled() runs when
// bootstrap resolves. By the time either is called, the dynamic
// import has resolved.
let cuelumeModule: typeof import("cuelume") | null = null;
void import("cuelume")
  .then((m) => {
    cuelumeModule = m;
  })
  .catch(() => {
    /* cuelume optional */
  });

const DEFAULT_VOLUME = 0.5;

/** Applies the enabled/disabled state to cuelume. */
export function applySoundsEnabled(enabled: boolean): void {
  if (!cuelumeModule) return;
  cuelumeModule.setEnabled(enabled);
  if (enabled) {
    cuelumeModule.setVolume(DEFAULT_VOLUME);
  }
}

/** Call once on app mount: wires data-cuelume-* attributes. */
export function initSounds(): void {
  if (!cuelumeModule) return;
  cuelumeModule.bind();
}
