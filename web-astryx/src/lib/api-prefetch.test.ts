import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// ── Test doubles ──

// Minimal Response-shaped object — prefetchShowDetail's pre-fired path only
// touches ok, headers.get("ETag"), and clone().json().
function fakeResponse(body: unknown, etag: string | null = null): Response {
  return {
    ok: true,
    headers: { get: (name: string) => (name === "ETag" ? etag : null) },
    clone: () => ({ json: async () => body }),
  } as unknown as Response;
}

// A promise we can resolve manually, to hold a prefetch "in flight".
function deferred(): { promise: Promise<Response>; resolve: (r: Response) => void } {
  let resolve!: (r: Response) => void;
  const promise = new Promise<Response>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

// api.ts reads window.__mizuPrefetch at module load — install the controlled
// pre-fired promises first, then import a fresh module instance so the
// module-level `prefetch` const captures them.
async function loadApiWithPrefetch(
  detailId: string,
  show: Promise<Response>,
  episodes: Promise<Response>,
) {
  (globalThis as unknown as { window: { __mizuPrefetch?: unknown } }).window.__mizuPrefetch = {
    detailId,
    show,
    episodes,
  };
  vi.resetModules();
  return await import("./api");
}

// Advance fake timers by 0 to drain the whole promise chain (prefired
// .then → clone().json() → Promise.all → cache write) deterministically.
async function flush() {
  for (let i = 0; i < 5; i++) {
    await vi.advanceTimersByTimeAsync(0);
  }
}

describe("prefetchShowDetail detail-cache write-back", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    delete (globalThis as unknown as { window: { __mizuPrefetch?: unknown } }).window
      .__mizuPrefetch;
    vi.useRealTimers();
  });

  it("does not write the stale snapshot back after clearCachedShowDetail ran mid-flight", async () => {
    const show = deferred();
    const episodes = deferred();
    const api = await loadApiWithPrefetch("5", show.promise, episodes.promise);

    // Hard-refresh flow: the prefetch fires at bundle load, the detail page
    // mounts while it's still in flight, fires its own getShow/getEpisodes
    // (which reuse the in-flight promises), then clears the prefetch caches.
    api.prefetchShowDetail(5);
    api.clearCachedShowDetail(5);

    // The stale prefetch resolves after the clear.
    show.resolve(fakeResponse({ id: 5, sonarr_series_id: 1, sonarr_title: "Stale" }));
    episodes.resolve(fakeResponse([]));
    await flush();

    // A later loadShowData (after a save) must NOT be served the resurrected
    // pre-save snapshot via getShow/getEpisodes.
    expect(api.getCachedShowDetail(5)).toBeNull();
  });

  it("still populates the detail cache when nothing cleared it", async () => {
    const show = deferred();
    const episodes = deferred();
    const api = await loadApiWithPrefetch("5", show.promise, episodes.promise);

    api.prefetchShowDetail(5);
    show.resolve(fakeResponse({ id: 5, sonarr_series_id: 1, sonarr_title: "Fresh" }));
    episodes.resolve(fakeResponse([]));
    await flush();

    const cached = api.getCachedShowDetail(5) as unknown as
      | { show: { sonarr_title: string } }
      | null;
    expect(cached?.show.sonarr_title).toBe("Fresh");
  });
});
