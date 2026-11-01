import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ARTWORK_REFETCH_DELAY_MS, queryKeys, useShowArtworkRefetch } from "./queries";
import type { Show } from "./schemas";

// ── Test doubles ──

function makeShow(overrides: Partial<Show> = {}): Show {
  return {
    id: 5,
    sonarr_series_id: 101,
    sonarr_title: "Frieren",
    sync_watermark: 0,
    enabled: true,
    filler_action: "keep",
    mixed_action: "skip",
    anime_canon_action: "keep",
    watermark_mode: "auto",
    ...overrides,
  };
}

// A promise we can resolve manually, to hold an invalidateQueries call "in
// flight" while the test drives a re-schedule or an unmount.
function deferred(): { promise: Promise<void>; resolve: () => void } {
  let resolve!: () => void;
  const promise = new Promise<void>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

// A harness that renders the hook and hands its schedule function to the
// test, so we can drive it without a query-observing child component.
let schedule: ((showId: string | number) => void) | null = null;

function Harness() {
  schedule = useShowArtworkRefetch();
  return createElement("div");
}

let container: HTMLDivElement;
let root: Root;
let qc: QueryClient;
let invalidateSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.useFakeTimers();
  qc = new QueryClient();
  invalidateSpy = vi.spyOn(qc, "invalidateQueries");
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  // react-query's notify timers may leave a pending scheduled update, and
  // React 19 warns if the unmount that flushes it isn't wrapped in act.
  act(() => {
    root.unmount();
  });
  container.remove();
  schedule = null;
  invalidateSpy.mockRestore();
  vi.useRealTimers();
});

async function renderHarness() {
  await act(async () => {
    root.render(createElement(QueryClientProvider, { client: qc }, createElement(Harness)));
  });
}

// Advance fake timers and let the timer callback's async work settle
// (invalidateQueries promise chain + react-query's zero-delay notifications)
// so every state update still happens inside act(). advanceTimersByTimeAsync
// fires only timers due within the window (unlike runAllTimersAsync, which
// would fire future timers too) and flushes microtasks between firings; the
// zero-advances drain any notify timers the promise chain schedules.
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(0);
  });
}

describe("useShowArtworkRefetch", () => {
  it("waits the full delay, then invalidates the show query and patches the cached shows list", async () => {
    // Seed the cache the way the mutation onSuccess flow leaves it: a fresh
    // show query (with artwork) and a shows list that still has the old entry.
    qc.setQueryData(queryKeys.show(5), makeShow({ artwork_poster_path: "new/poster.jpg" }));
    qc.setQueryData(queryKeys.shows, [makeShow({ artwork_poster_path: "old/poster.jpg" })]);
    await renderHarness();

    // Before the delay elapses: nothing happens.
    await act(async () => {
      schedule!(5);
    });
    await advance(ARTWORK_REFETCH_DELAY_MS - 1);
    expect(invalidateSpy).not.toHaveBeenCalled();
    expect(qc.getQueryData<Show[]>(queryKeys.shows)![0].artwork_poster_path).toBe("old/poster.jpg");

    // The delay elapses: the show query is invalidated and its data is
    // patched into the cached shows list. (The patch premise is that the
    // show query has no mounted observer, so invalidateQueries resolves
    // without refetching and getQueryData returns the seeded data.)
    await advance(1);
    expect(invalidateSpy).toHaveBeenCalledTimes(1);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.show(5) });
    expect(qc.getQueryData<Show[]>(queryKeys.shows)![0].artwork_poster_path).toBe("new/poster.jpg");
  });

  it("dedupes rapid re-schedules of the same show", async () => {
    qc.setQueryData(queryKeys.show(7), makeShow({ id: 7 }));
    await renderHarness();

    // Schedule, wait most of the delay, then re-schedule — the second call
    // cancels the first timer, so the first deadline passes silently.
    await act(async () => {
      schedule!(7);
    });
    await advance(ARTWORK_REFETCH_DELAY_MS - 2000);
    await act(async () => {
      schedule!(7);
    });
    await advance(ARTWORK_REFETCH_DELAY_MS - 2000);
    expect(invalidateSpy).not.toHaveBeenCalled();

    // Only the (re-scheduled) timer fires, exactly once.
    await advance(2000);
    expect(invalidateSpy).toHaveBeenCalledTimes(1);
  });

  it("keeps independent timers for different shows", async () => {
    qc.setQueryData(queryKeys.show(1), makeShow({ id: 1 }));
    qc.setQueryData(queryKeys.show(2), makeShow({ id: 2 }));
    await renderHarness();

    await act(async () => {
      schedule!(1);
      schedule!(2);
    });
    await advance(ARTWORK_REFETCH_DELAY_MS + 10);
    expect(invalidateSpy).toHaveBeenCalledTimes(2);
  });

  it("cancels the pending refetch when the component unmounts", async () => {
    qc.setQueryData(queryKeys.show(9), makeShow({ id: 9 }));
    await renderHarness();

    await act(async () => {
      schedule!(9);
    });
    await act(async () => {
      root.unmount();
    });

    // Even long after the delay would have elapsed, nothing fires.
    await advance(ARTWORK_REFETCH_DELAY_MS * 2);
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("ignores the stale patch when a re-schedule lands mid-flight", async () => {
    // Seed the detail query with the pre-refetch snapshot and the shows list
    // with the same value.
    qc.setQueryData(queryKeys.show(7), makeShow({ id: 7, sonarr_title: "Initial" }));
    qc.setQueryData(queryKeys.shows, [makeShow({ id: 7, sonarr_title: "Initial" })]);
    await renderHarness();

    // The first timer fires and its invalidateQueries stays in flight.
    const { promise: inFlightA, resolve: resolveA } = deferred();
    invalidateSpy.mockReturnValueOnce(inFlightA);
    await act(async () => {
      schedule!(7);
    });
    await advance(ARTWORK_REFETCH_DELAY_MS);
    expect(invalidateSpy).toHaveBeenCalledTimes(1);

    // A re-schedule for the same show lands while that refetch is still
    // in flight…
    await act(async () => {
      schedule!(7);
    });

    // …and a fresher snapshot arrives in the shows list before the stale
    // refetch resolves.
    qc.setQueryData(queryKeys.shows, [makeShow({ id: 7, sonarr_title: "Fresh" })]);

    // The stale refetch resolves now — its patch must not clobber "Fresh".
    await act(async () => {
      resolveA();
    });

    const list = qc.getQueryData<Show[]>(queryKeys.shows)!;
    expect(list[0].sonarr_title).toBe("Fresh");
  });

  it("does not patch the shows list when the component unmounts mid-flight", async () => {
    qc.setQueryData(queryKeys.show(11), makeShow({ id: 11, sonarr_title: "Initial" }));
    qc.setQueryData(queryKeys.shows, [makeShow({ id: 11, sonarr_title: "Initial" })]);
    await renderHarness();

    // Fire the timer and keep its invalidateQueries in flight.
    const { promise: inFlightA, resolve: resolveA } = deferred();
    invalidateSpy.mockReturnValueOnce(inFlightA);
    await act(async () => {
      schedule!(11);
    });
    await advance(ARTWORK_REFETCH_DELAY_MS);
    expect(invalidateSpy).toHaveBeenCalledTimes(1);

    // A fresher snapshot lands, then the component unmounts before the
    // in-flight refetch resolves.
    qc.setQueryData(queryKeys.shows, [makeShow({ id: 11, sonarr_title: "Fresh" })]);
    await act(async () => {
      root.unmount();
    });
    await act(async () => {
      resolveA();
    });

    const list = qc.getQueryData<Show[]>(queryKeys.shows)!;
    expect(list[0].sonarr_title).toBe("Fresh");
  });
});
