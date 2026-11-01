import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";

// ── Module mocks ──
// The page talks to the backend through refreshAllArtwork and
// getArtworkRefreshStatus, to the notification store, and to the react-query
// client (loadLanding invalidates queries on completion). Mock those so the
// tests drive the poll loop purely via fake timers and controlled status
// responses. The lazy AddAnimeDialog chunk is replaced with a null stub:
// the astryx Dialog renders its children even when closed, so the lazy
// import would otherwise fire (and pull the 700-line wizard) on every render.

const { addNotificationMock } = vi.hoisted(() => ({ addNotificationMock: vi.fn() }));

vi.mock("../lib/api", () => ({
  refreshAllArtwork: vi.fn().mockResolvedValue({ status: "started" }),
  getArtworkRefreshStatus: vi.fn(),
  getArtworkPosterUrl: vi.fn(),
  getArtworkBannerUrl: vi.fn(),
  getArtworkLogoUrl: vi.fn(),
  getSonarrAddOptions: vi.fn(),
  getSonarrSeries: vi.fn(),
  getSettings: vi.fn(),
  prefetchEpisodes: vi.fn(),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

// NOTE: the data values must be module-level constants. ShowsPage feeds
// sonarrSeriesQuery.data into an effect dep and calls setAllSeries(data) —
// a fresh array per render would change the dep every render and loop
// (setAllSeries of a new reference never bails out).
const EMPTY_SHOWS: never[] = [];
const EMPTY_SERIES: never[] = [];
vi.mock("../lib/queries", () => ({
  useBootstrap: () => ({ data: null, isPending: false, isFetched: true }),
  useShows: () => ({ data: EMPTY_SHOWS, isPending: false, isFetched: true }),
  useSettings: () => ({ data: null, isPending: false, isFetched: true }),
  useSonarrSeries: () => ({ data: EMPTY_SERIES, isPending: false, isFetched: true }),
  useSyncLogs: () => ({ data: undefined, isPending: false, isFetched: true }),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => addNotificationMock,
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({ invalidateQueries: vi.fn().mockResolvedValue(undefined) }),
}));

vi.mock("./AddAnimeDialog", () => ({
  AddAnimeDialog: () => null,
}));

import { getArtworkRefreshStatus, refreshAllArtwork } from "../lib/api";
import { ShowsPage } from "./ShowsPage";

const mockedGetStatus = vi.mocked(getArtworkRefreshStatus);
const mockedRefreshAll = vi.mocked(refreshAllArtwork);

// Mirror the polling constants in ShowsPage.tsx. Kept as literals (not
// imported) so the failing-first stash check exercises the real behavior
// rather than a missing export.
const POLL_INTERVAL = 3000;
const FAST_POLLS = 100;
const WATCHDOG_INTERVAL = 30_000;
const ERROR_BACKOFF_BASE = 3000;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.useFakeTimers();
  // happy-dom defaults visibilityState to "visible", but pin it so the
  // visibility-recovery handler's guard behaves deterministically.
  Object.defineProperty(document, "visibilityState", {
    value: "visible",
    configurable: true,
  });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  addNotificationMock.mockClear();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.useRealTimers();
  vi.clearAllMocks();
});

async function renderPage() {
  await act(async () => {
    root.render(createElement(MemoryRouter, null, createElement(ShowsPage)));
  });
}

async function clickRefreshArtwork() {
  await act(async () => {
    const btn = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Refresh artwork"),
    );
    if (!btn) throw new Error("Refresh artwork button not found");
    btn.click();
  });
}

function refreshButtonLabel(): string | undefined {
  return Array.from(container.querySelectorAll("button"))
    .find((b) => b.textContent?.includes("Refresh artwork"))
    ?.textContent?.trim();
}

function completeNotifications(): number {
  return addNotificationMock.mock.calls.filter((c) => c[0] === "Artwork refresh complete").length;
}

describe("ShowsPage artwork-refresh polling", () => {
  it("keeps polling through a transient status-check error instead of abandoning the wait", async () => {
    let calls = 0;
    mockedGetStatus.mockImplementation(async () => {
      calls++;
      if (calls === 1) throw new Error("network blip");
      return { refreshing: calls < 3 };
    });
    await renderPage();
    await clickRefreshArtwork(); // triggers refreshAllArtwork, schedules the first poll

    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // call 1 — error → backoff retry
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ERROR_BACKOFF_BASE); // call 2 — still refreshing
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // call 3 — terminal
    });
    // Pre-fix: the single error ends tracking — no further status checks,
    // no completion notification, button back to normal after the 8s fallback.
    expect(calls).toBe(3);
    expect(completeNotifications()).toBe(1);
    expect(refreshButtonLabel()).toBe("Refresh artwork");
  });

  it("drops to a slow watchdog cadence after the fast phase instead of burning requests", async () => {
    mockedGetStatus.mockResolvedValue({ refreshing: true });
    await renderPage();
    await clickRefreshArtwork();

    // Fast phase: first poll at 3s, then FAST_POLLS fast polls (~5 min).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL + FAST_POLLS * POLL_INTERVAL);
    });
    const afterFast = mockedGetStatus.mock.calls.length;
    expect(afterFast).toBeGreaterThan(FAST_POLLS); // still tracking the refresh

    // Still running — the next poll must wait for the 30s watchdog, not fire
    // at the fast 3s cadence (pre-fix: 3s forever → a call fires here).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL);
    });
    expect(mockedGetStatus.mock.calls.length).toBe(afterFast);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(WATCHDOG_INTERVAL - POLL_INTERVAL);
    });
    expect(mockedGetStatus.mock.calls.length).toBeGreaterThan(afterFast);
  });

  it("stops polling once the refresh reaches a terminal state", async () => {
    let calls = 0;
    mockedGetStatus.mockImplementation(async () => {
      calls++;
      return { refreshing: calls < 2 };
    });
    await renderPage();
    await clickRefreshArtwork();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // call 1 — still refreshing
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // call 2 — terminal → notify
    });
    expect(completeNotifications()).toBe(1);
    const callsAtTerminal = mockedGetStatus.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000); // no further polls expected
    });
    expect(mockedGetStatus.mock.calls.length).toBe(callsAtTerminal);
  });

  it("immediately checks status and resumes tracking when the tab regains focus during a refresh", async () => {
    mockedGetStatus.mockResolvedValue({ refreshing: true });
    await renderPage();
    await clickRefreshArtwork();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // one poll — still refreshing
    });
    const beforeFocus = mockedGetStatus.mock.calls.length;

    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });

    // The focus handler must check status immediately (pre-fix: no listener).
    expect(mockedGetStatus.mock.calls.length).toBeGreaterThan(beforeFocus);
  });

  it("does not spawn a duplicate polling chain when focus lands while a poll is in flight", async () => {
    let calls = 0;
    let resolveParked!: (v: { refreshing: boolean }) => void;
    const parked = new Promise<{ refreshing: boolean }>((r) => (resolveParked = r));
    mockedGetStatus.mockImplementation(async () => {
      calls++;
      if (calls === 2) return parked; // park the poll that focus will interrupt
      return { refreshing: true };
    });

    await renderPage();
    await clickRefreshArtwork();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // call 1 — running
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // call 2 — parked mid-flight
    });
    await act(async () => {
      // Focus while the poll is mid-await: an immediate check (call 3) starts
      // a fresh chain. The old chain must not keep rescheduling itself.
      window.dispatchEvent(new Event("focus"));
    });
    await act(async () => {
      resolveParked({ refreshing: true }); // old poll resumes — must bail via the chain guard
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL); // fresh chain's first tick — call 4
    });
    // Exactly one new poll: a surviving superseded chain would add a 5th call
    // in the same tick.
    expect(calls).toBe(4);
  });

  it("keeps polling under StrictMode double-mounting", async () => {
    mockedGetStatus.mockResolvedValue({ refreshing: true });
    await act(async () => {
      root.render(
        createElement(
          StrictMode,
          null,
          createElement(MemoryRouter, null, createElement(ShowsPage)),
        ),
      );
    });
    await clickRefreshArtwork();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL + POLL_INTERVAL);
    });
    // StrictMode's simulated unmount bumps the poll chain; the click must
    // still start a chain that polls normally.
    expect(mockedGetStatus.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("does not start a second poll chain when focus lands while the refresh trigger is in flight", async () => {
    // Park the trigger response: the click starts handleRefreshAllArtwork and
    // awaits refreshAllArtwork, while the focus listener (registered as soon
    // as artworkRefreshing becomes true) is already active.
    let resolveTrigger!: (v: { status: string }) => void;
    const triggerPending = new Promise<{ status: string }>((r) => (resolveTrigger = r));
    mockedRefreshAll.mockReturnValueOnce(triggerPending);
    mockedGetStatus.mockResolvedValue({ refreshing: true });

    await renderPage();
    await clickRefreshArtwork(); // refreshAllArtwork parked mid-flight
    await act(async () => {
      // Focus while the trigger is pending: resume() starts a fresh chain
      // (status call 1) and supersedes the clicked chain.
      window.dispatchEvent(new Event("focus"));
    });
    await act(async () => {
      resolveTrigger({ status: "started" });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL);
    });
    // Exactly one poll fires (the focus chain's tick). Without the supersede
    // guard on the trigger's success path, the clicked chain would also
    // schedule a poll here, adding a 3rd status call.
    expect(mockedGetStatus.mock.calls.length).toBe(2);
  });
});
