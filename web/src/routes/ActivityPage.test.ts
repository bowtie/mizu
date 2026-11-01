import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// ── Module mocks ──
// The page reads the sync history through the real query hooks; only the
// HTTP layer and the notification store are mocked. Fake timers drive the
// polling cadence.

vi.mock("../lib/api", () => ({
  getSyncLogs: vi.fn(),
  triggerSync: vi.fn(),
  getShows: vi.fn().mockResolvedValue([]),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => vi.fn(),
}));

import { getSyncLogs, triggerSync } from "../lib/api";
import type { SyncLog, SyncLogResponse } from "../lib/types";
import { ActivityPage } from "./ActivityPage";

const mockedGetSyncLogs = vi.mocked(getSyncLogs);
const mockedTriggerSync = vi.mocked(triggerSync);

const NOW = new Date("2026-11-01T12:00:00Z").getTime();

function makeLog(status: string, id: number, startedMinutesAgo = 0): SyncLog {
  return {
    id,
    status,
    trigger: "manual",
    shows_processed: 1,
    episodes_updated: 0,
    errors: "",
    started_at: new Date(NOW - startedMinutesAgo * 60_000).toISOString(),
    completed_at: undefined,
    items: [],
  };
}

function logPage(logs: SyncLog[], hasMore = false): SyncLogResponse {
  return { items: logs, has_more: hasMore, next_offset: logs.length };
}

let container: HTMLDivElement;
let root: Root;
let queryClient: QueryClient;

beforeEach(() => {
  vi.useFakeTimers({ now: NOW });
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  mockedGetSyncLogs.mockReset();
  mockedTriggerSync.mockReset();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  queryClient.clear();
  vi.useRealTimers();
});

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

async function renderPage() {
  await act(async () => {
    root.render(
      createElement(QueryClientProvider, { client: queryClient }, createElement(ActivityPage)),
    );
  });
  await advance(0);
}

function findButton(label: string) {
  return Array.from(container.querySelectorAll("button")).find((b) =>
    b.textContent?.includes(label),
  );
}

describe("ActivityPage", () => {
  it("polls every 2s while a sync runs and stops once it finishes", async () => {
    mockedGetSyncLogs
      .mockResolvedValueOnce(logPage([makeLog("running", 1)]))
      .mockResolvedValueOnce(logPage([makeLog("running", 1)]))
      .mockResolvedValue(logPage([makeLog("success", 1)]));
    await renderPage();
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(1);

    await advance(2000);
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(2);
    await advance(2000);
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(3);

    // The run finished: the "finished" invalidation refetches once, then
    // polling stops.
    await advance(2000);
    const settled = mockedGetSyncLogs.mock.calls.length;
    await advance(30_000);
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(settled);
  });

  it("drops to a 30s watchdog for a run older than five minutes", async () => {
    mockedGetSyncLogs.mockResolvedValue(logPage([makeLog("running", 1, 6)]));
    await renderPage();
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(1);

    await advance(29_000);
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(1);
    await advance(1_000);
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(2);
  });

  it("keeps polling after Sync now until the new run's log appears", async () => {
    mockedGetSyncLogs.mockResolvedValue(logPage([makeLog("success", 1, 60)]));
    mockedTriggerSync.mockResolvedValue({ status: "sync triggered" });
    await renderPage();
    expect(mockedGetSyncLogs).toHaveBeenCalledTimes(1);

    // The run's row isn't there yet for the first polls...
    await act(async () => {
      findButton("Sync now")!.click();
    });
    await advance(0);
    expect(mockedTriggerSync).toHaveBeenCalledTimes(1);
    const afterTrigger = mockedGetSyncLogs.mock.calls.length;
    await advance(2000);
    expect(mockedGetSyncLogs.mock.calls.length).toBeGreaterThan(afterTrigger);

    // ...then it appears and is tracked.
    mockedGetSyncLogs.mockResolvedValue(
      logPage([makeLog("running", 2), makeLog("success", 1, 60)]),
    );
    await advance(2000);
    expect(container.textContent).toContain("Running");
  });

  it("loads the next page with Load more", async () => {
    mockedGetSyncLogs
      .mockResolvedValueOnce(logPage([makeLog("success", 1, 5)], true))
      .mockResolvedValueOnce({
        items: [makeLog("failed", 2, 90)],
        has_more: false,
        next_offset: 2,
      });
    await renderPage();

    await act(async () => {
      findButton("Load more")!.click();
    });
    await advance(0);

    expect(mockedGetSyncLogs).toHaveBeenLastCalledWith(8, 1);
    expect(findButton("Load more")).toBeUndefined();
  });
});
