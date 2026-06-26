export const POSTER_LOADING = 'loading';
export const POSTER_LOADED = 'loaded';
export const POSTER_FAILED = 'failed';

export function getPosterStateKey(show) {
  return `${show?.id ?? 'unknown'}::${show?.poster ?? ''}`;
}

export function createPosterStateMap(shows, previousStates = {}) {
  return Object.fromEntries(
    (shows || []).map((show) => {
      const key = getPosterStateKey(show);
      const nextState = !show?.poster
        ? POSTER_FAILED
        : previousStates[key] === POSTER_LOADED || previousStates[key] === POSTER_FAILED
          ? previousStates[key]
          : POSTER_LOADING;

      return [key, nextState];
    }),
  );
}

export function markPosterLoaded(previousStates, show) {
  return {
    ...previousStates,
    [getPosterStateKey(show)]: POSTER_LOADED,
  };
}

export function markPosterFailed(previousStates, show) {
  return {
    ...previousStates,
    [getPosterStateKey(show)]: POSTER_FAILED,
  };
}

export function shouldShowPoster(state) {
  return state === POSTER_LOADED;
}

export function resolvePosterElementState({ hasPoster, complete, naturalWidth }) {
  if (!hasPoster) return POSTER_FAILED;
  if (!complete) return POSTER_LOADING;
  return naturalWidth > 0 ? POSTER_LOADED : POSTER_FAILED;
}

export function createPosterMonogram(title) {
  return (title || '')
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0] || '')
    .join('')
    .toUpperCase();
}
