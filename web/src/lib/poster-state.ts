export const POSTER_LOADING = 'loading' as const;
export const POSTER_LOADED = 'loaded' as const;
export const POSTER_FAILED = 'failed' as const;

export type PosterState = typeof POSTER_LOADING | typeof POSTER_LOADED | typeof POSTER_FAILED;

export interface PosterSource {
  id?: string | number;
  poster?: string;
}

export function getPosterStateKey(show: PosterSource | null | undefined): string {
  return `${show?.id ?? 'unknown'}::${show?.poster ?? ''}`;
}

export function createPosterStateMap(shows: PosterSource[] = [], previousStates: Record<string, string> = {}): Record<string, PosterState> {
  return Object.fromEntries(
    shows.map((show) => {
      const key = getPosterStateKey(show);
      const previous = previousStates[key];
      let nextState: PosterState;
      if (!show?.poster) {
        nextState = POSTER_FAILED;
      } else if (previous === POSTER_LOADED || previous === POSTER_FAILED) {
        nextState = previous;
      } else {
        nextState = POSTER_LOADING;
      }

      return [key, nextState];
    }),
  );
}

export function markPosterLoaded(previousStates: Record<string, string>, show: PosterSource | null | undefined): Record<string, string> {
  return {
    ...previousStates,
    [getPosterStateKey(show)]: POSTER_LOADED,
  };
}

export function markPosterFailed(previousStates: Record<string, string>, show: PosterSource | null | undefined): Record<string, string> {
  return {
    ...previousStates,
    [getPosterStateKey(show)]: POSTER_FAILED,
  };
}

export function shouldShowPoster(state: PosterState): boolean {
  return state === POSTER_LOADED;
}

export function resolvePosterElementState({ hasPoster, complete, naturalWidth }: { hasPoster: boolean; complete: boolean; naturalWidth: number }): PosterState {
  if (!hasPoster) return POSTER_FAILED;
  if (!complete) return POSTER_LOADING;
  return naturalWidth > 0 ? POSTER_LOADED : POSTER_FAILED;
}

export function createPosterMonogram(title: string = ''): string {
  return title
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0] || '')
    .join('')
    .toUpperCase();
}
