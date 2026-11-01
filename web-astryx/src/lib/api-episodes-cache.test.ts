import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// ── Test doubles ──

function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void } {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

// Minimal Response-shaped object — requestEpisodesWithETag only touches
// status, ok, headers.get("ETag"), and json().
function fakeResponse(etag: string | null, json: Promise<unknown>): Response {
  return {
    ok: true,
    status: 200,
    headers: { get: (name: string) => (name === "ETag" ? etag : null) },
    json: () => json,
  } as unknown as Response;
}

function fake304(): Response {
  return {
    ok: false,
    status: 304,
    headers: { get: () => null },
    json: async () => ({}),
  } as unknown as Response;
}

// Import a fresh api module (fresh module-level caches) with no pre-fired
// responses wired up, so getEpisodes falls through to requestEpisodesWithETag
// → the stubbed global fetch.
async function loadFreshApi() {
  delete (globalThis as unknown as { window: { __mizuPrefetch?: unknown } }).window.__mizuPrefetch;
  vi.resetModules();
  return await import("./api");
}

// Advance fake timers by 0 to drain the whole promise chain deterministically.
async function flush() {
  for (let i = 0; i < 5; i++) {
    await vi.advanceTimersByTimeAsync(0);
  }
}

describe("episode ETag/data cache write-back", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    delete (globalThis as unknown as { window: { __mizuPrefetch?: unknown } }).window
      .__mizuPrefetch;
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("keeps the newest response's ETag/data pair when an older response's split writes land last", async () => {
    const api = await loadFreshApi();

    const olderFetch = deferred<Response>();
    const olderJson = deferred<unknown>();
    const newerFetch = deferred<Response>();
    let capturedEtag: string | undefined;

    const fetchMock = vi.fn();
    fetchMock
      .mockImplementationOnce(() => olderFetch.promise) // fetch A (older)
      .mockImplementationOnce(() => newerFetch.promise) // fetch B (newer)
      .mockImplementationOnce((_url: unknown, init: unknown) => {
        // Third call: revalidates with the cached ETag; the server says 304.
        capturedEtag = (init as { headers?: Record<string, string> } | undefined)?.headers?.[
          "If-None-Match"
        ];
        return Promise.resolve(fake304());
      });
    vi.stubGlobal("fetch", fetchMock);

    // Two concurrent episode fetches for the same key.
    const pA = api.getEpisodes(5);
    const pB = api.getEpisodes(5);

    // The older response resolves first: its ETag write lands immediately
    // (pre-fix), then it suspends on JSON parsing.
    olderFetch.resolve(fakeResponse("E_A", olderJson.promise));
    await flush();

    // The newer response resolves fully: ETag E_B + data B.
    newerFetch.resolve(fakeResponse("E_B", Promise.resolve([{ id: "B" }])));
    await flush();

    // The older response's JSON parse lands last. Pre-fix it overwrites the
    // data while the newer ETag stays — the mismatched {ETag: E_B, data: D_A}
    // pair that the server keeps validating on 304.
    olderJson.resolve([{ id: "A" }]);
    await flush();
    await pA;
    await pB;

    // A later request revalidates E_B (304) and must be served the NEWER
    // data — not the stale D_A the split write left behind.
    const result = await api.getEpisodes(5);
    expect(capturedEtag).toBe("E_B");
    expect(result).toEqual([{ id: "B" }]);
  });

  it("does not resurrect the ETag/data caches when a stale fetch resolves after clearCachedShowDetail", async () => {
    const api = await loadFreshApi();

    const staleFetch = deferred<Response>();
    const staleJson = deferred<unknown>();
    let capturedEtag: string | undefined;

    const fetchMock = vi.fn();
    fetchMock
      .mockImplementationOnce(() => staleFetch.promise)
      .mockImplementationOnce((_url: unknown, init: unknown) => {
        // Post-clear call: must go out WITHOUT If-None-Match — if the stale
        // fetch resurrected the cache, it would send the old ETag.
        capturedEtag = (init as { headers?: Record<string, string> } | undefined)?.headers?.[
          "If-None-Match"
        ];
        return Promise.resolve(fakeResponse("E_FRESH", Promise.resolve([{ id: "fresh" }])));
      });
    vi.stubGlobal("fetch", fetchMock);

    const pStale = api.getEpisodes(5);
    // The cache is cleared while the fetch is still in flight (hard-refresh
    // flow: prefetch fired, page clears after firing its own fetches).
    api.clearCachedShowDetail(5);

    // The stale fetch resolves after the clear — pre-fix it writes its
    // ETag + data back into the caches it just had wiped.
    staleFetch.resolve(fakeResponse("E_OLD", staleJson.promise));
    staleJson.resolve([{ id: "stale" }]);
    await flush();
    await pStale;

    const result = await api.getEpisodes(5);
    // Pre-fix: If-None-Match: E_OLD was resurrected → this assertion fails.
    expect(capturedEtag).toBeUndefined();
    expect(result).toEqual([{ id: "fresh" }]);
  });

  it("still serves cached data on 304 for a single fetch (control)", async () => {
    const api = await loadFreshApi();
    const fetchMock = vi.fn();
    fetchMock
      .mockImplementationOnce(() =>
        Promise.resolve(fakeResponse("E1", Promise.resolve([{ id: "one" }]))),
      )
      .mockImplementationOnce(() => Promise.resolve(fake304()));
    vi.stubGlobal("fetch", fetchMock);

    // First call populates the ETag + data caches and returns the payload.
    const first = await api.getEpisodes(5);
    expect(first).toEqual([{ id: "one" }]);

    // Second call revalidates E1, gets 304, and serves the cached data —
    // the happy-path cache behavior must be preserved.
    const second = await api.getEpisodes(5);
    expect(second).toEqual([{ id: "one" }]);
  });
});
