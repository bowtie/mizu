import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";

// ── Module mocks ──
// The component talks to the backend through getSyncLogs and triggerSync,
// and to the notification store through useNotificationStore. Mock all
// three so the tests drive polling purely via fake timers and controlled
// getSyncLogs responses.

vi.mock("../lib/api", () => ({
  getSyncLogs: vi.fn(),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../lib/queries", () => ({
  useTriggerSync: () => ({
    mutateAsync: vi.fn().mockResolvedValue(undefined),
  }),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => vi.fn(),
}));

import { getSyncLogs } from "../lib/api";
import type { SyncLog, SyncLogResponse } from "../lib/types";
import { ActivityPage } from "./ActivityPage";

const mockedGetSyncLogs = vi.mocked(getSyncLogs);

// Mirror the polling constants in ActivityPage.tsx. Kept as literals (not
// imported) so the failing-first stash check exercises the real behavior
// rather than a missing export.
const FIRST_DELAY = 1800;
const INTERVAL = 2000;
const FAST_ATTEMPTS = 150; // pre-fix cap was 20 (~40s)
const SLOW_INTERVAL = 30_000;

function makeLog(status: string, id: number): SyncLog {
  return {
    id,
    status,
    trigger: "manual",
    shows_processed: 1,
    episodes_updated: 0,
    errors: "",
    started_at: undefined,
    completed_at: undefined,
    items: [],
  };
}

function logPage(...logs: SyncLog[]): SyncLogResponse {
  return { items: logs, has_more: false, next_offset: logs.length };
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.useFakeTimers();
  // happy-dom defaults visibilityState to "visible", but pin it so the
  // focus-recovery handler's guard behaves deterministically.
  Object.defineProperty(document, "visibilityState", {
    value: "visible",
    configurable: true,
  });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.useRealTimers();
  vi.clearAllMocks();
});

async function renderPage() {
  await act(async () => {
    root.render(createElement(ActivityPage));
  });
}

async function clickSyncNow() {
  await act(async () => {
    const btn = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Sync now"),
    );
    if (!btn) throw new Error("Sync now button not found");
    btn.click();
  });
}

describe("ActivityPage sync polling", () => {
  it("keeps polling while the sync stays running, past the old ~40s cap", async () => {
    mockedGetSyncLogs.mockResolvedValue(logPage(makeLog("running", 1)));
    await renderPage(); // initial load
    await clickSyncNow(); // schedules the first poll

    // 25 polls at 2s after the 1800ms first poll — well beyond the old
    // 20-attempt cap, so the page must still be tracking the sync.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY + 25 * INTERVAL);
    });
    // 1 (mount) + 26 polls; pre-fix it stops at 21 total.
    expect(mockedGetSyncLogs.mock.calls.length).toBeGreaterThanOrEqual(27);
  });

  it("stops polling once the sync reaches a terminal state", async () => {
    let calls = 0;
    mockedGetSyncLogs.mockImplementation(async () => {
      calls++;
      return logPage(makeLog(calls <= 2 ? "running" : "success", 1));
    });
    await renderPage(); // call 1 — running
    await clickSyncNow();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY); // call 2 — running
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(INTERVAL); // call 3 — success → stop
    });
    const callsAtTerminal = mockedGetSyncLogs.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000); // no further polls expected
    });
    expect(mockedGetSyncLogs.mock.calls.length).toBe(callsAtTerminal);
  });

  it("keeps tracking at a slow watchdog cadence after the fast phase instead of freezing", async () => {
    mockedGetSyncLogs.mockResolvedValue(logPage(makeLog("running", 1)));
    await renderPage();
    await clickSyncNow();

    // Fast phase: first poll at 1800ms, then FAST_ATTEMPTS × 2s.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY + FAST_ATTEMPTS * INTERVAL);
    });
    const afterFast = mockedGetSyncLogs.mock.calls.length; // 1 mount + 151 polls
    expect(afterFast).toBeGreaterThan(1 + FAST_ATTEMPTS - 20); // sanity: cap removed

    // Sync still running — the 30s watchdog must fire (pre-fix: frozen).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(SLOW_INTERVAL);
    });
    expect(mockedGetSyncLogs.mock.calls.length).toBeGreaterThan(afterFast);
  });

  it("refreshes and resumes polling when the tab regains focus while a sync is running", async () => {
    mockedGetSyncLogs.mockResolvedValue(logPage(makeLog("running", 1)));
    await renderPage();
    await clickSyncNow();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY); // one poll — still running
    });
    const beforeFocus = mockedGetSyncLogs.mock.calls.length;

    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });

    // The focus handler must reload the logs immediately (pre-fix: no
    // listener, so nothing happens until the page remounts).
    expect(mockedGetSyncLogs.mock.calls.length).toBeGreaterThan(beforeFocus);
  });

  it("does not spawn a duplicate polling chain when focus lands while a poll is in flight", async () => {
    let calls = 0;
    let resolveInflight!: (v: SyncLogResponse) => void;
    const inflight = new Promise<SyncLogResponse>((r) => (resolveInflight = r));
    mockedGetSyncLogs.mockImplementation(async () => {
      calls++;
      if (calls === 2) return inflight; // park the poll that focus will interrupt
      return logPage(makeLog("running", 1));
    });

    await renderPage(); // call 1 — running
    await clickSyncNow();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY); // call 2 — parked in flight
    });
    await act(async () => {
      // Focus while the poll is mid-await: reloads (call 3) and starts a
      // fresh chain. The old chain must not keep rescheduling itself.
      window.dispatchEvent(new Event("focus"));
    });
    await act(async () => {
      resolveInflight(logPage(makeLog("running", 1))); // old poll resumes
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY); // new chain's first tick — call 4
    });
    const afterNewChain = calls;
    // If the superseded chain survived, its own 2s tick fires 200ms later.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(INTERVAL - FIRST_DELAY);
    });
    expect(calls).toBe(afterNewChain); // pre-fix: the old chain adds a 5th call
  });

  it("keeps polling under StrictMode double-mounting (isMounted must reset on mount)", async () => {
    mockedGetSyncLogs.mockResolvedValue(logPage(makeLog("running", 1)));
    // main.tsx renders the app inside <StrictMode>, which in dev mounts,
    // cleans up, and mounts again — the cleanup sets isMounted=false. If it
    // is never reset on the second mount, every poll bails and the page
    // freezes on a stale "Running" log.
    await act(async () => {
      root.render(createElement(StrictMode, null, createElement(ActivityPage)));
    });
    expect(mockedGetSyncLogs.mock.calls.length).toBeGreaterThanOrEqual(2); // double mount loaded twice

    await clickSyncNow();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FIRST_DELAY + INTERVAL); // first + second poll
    });
    // Both mounts' load plus at least the first poll must have run.
    expect(mockedGetSyncLogs.mock.calls.length).toBeGreaterThanOrEqual(3);
  });
});
