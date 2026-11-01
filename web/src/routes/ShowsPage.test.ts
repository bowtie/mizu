import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// ── Module mocks ──
// The page reads through the real query hooks; only the HTTP layer and the
// notification store are mocked, and fake timers drive the artwork-refresh
// polling. The lazy AddAnimeDialog chunk is replaced with a null stub: the
// astryx Dialog renders its children even when closed, so the lazy import
// would otherwise pull the wizard on every render.

const { addNotificationMock } = vi.hoisted(() => ({ addNotificationMock: vi.fn() }));

const SHOW = {
  id: 1,
  sonarr_series_id: 7,
  sonarr_title: "Naruto",
  filler_action: "unmonitor",
  mixed_action: "monitor",
  anime_canon_action: "monitor",
  watermark_mode: "auto",
  sync_watermark: 0,
  enabled: true,
};

vi.mock("../lib/api", () => ({
  getShows: vi.fn(),
  getSettings: vi.fn().mockRejectedValue(new Error("not needed")),
  getSonarrSeries: vi.fn().mockResolvedValue([]),
  getSyncLogs: vi.fn().mockResolvedValue({ items: [] }),
  refreshAllArtwork: vi.fn(),
  getArtworkRefreshStatus: vi.fn(),
  getArtworkPosterUrl: vi.fn(),
  getArtworkBannerUrl: vi.fn(),
  getArtworkLogoUrl: vi.fn(),
  getEpisodes: vi.fn(),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => addNotificationMock,
}));

vi.mock("./AddAnimeDialog", () => ({
  AddAnimeDialog: () => null,
}));

import { getArtworkRefreshStatus, getShows, refreshAllArtwork } from "../lib/api";
import { ShowsPage } from "./ShowsPage";

const mockedGetShows = vi.mocked(getShows);
const mockedGetStatus = vi.mocked(getArtworkRefreshStatus);
const mockedRefreshAll = vi.mocked(refreshAllArtwork);

let container: HTMLDivElement;
let root: Root;
let queryClient: QueryClient;

beforeEach(() => {
  vi.useFakeTimers();
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  addNotificationMock.mockClear();
  mockedGetShows.mockReset();
  mockedGetShows.mockResolvedValue([SHOW] as never);
  mockedGetStatus.mockReset();
  mockedGetStatus.mockResolvedValue({ refreshing: false });
  mockedRefreshAll.mockReset();
  mockedRefreshAll.mockResolvedValue({ status: "refresh triggered" });
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
      createElement(
        QueryClientProvider,
        { client: queryClient },
        createElement(MemoryRouter, null, createElement(ShowsPage)),
      ),
    );
  });
  await advance(0);
}

function refreshButtonLabel(): string | undefined {
  return Array.from(container.querySelectorAll("button"))
    .find((b) => /Refresh artwork|Refreshing/.test(b.textContent ?? ""))
    ?.textContent?.trim();
}

async function clickRefreshArtwork() {
  await act(async () => {
    Array.from(container.querySelectorAll("button"))
      .find((b) => b.textContent?.includes("Refresh artwork"))!
      .click();
  });
  await advance(0);
}

function completeNotifications(): number {
  return addNotificationMock.mock.calls.filter((c) => c[0] === "Artwork refresh complete").length;
}

describe("ShowsPage", () => {
  it("renders the library from the shows query", async () => {
    await renderPage();
    expect(container.textContent).toContain("Naruto");
    expect(container.textContent).toContain("1 tracked");
  });

  it("polls an artwork refresh until it ends, then reports and reloads the library", async () => {
    await renderPage();
    mockedGetStatus
      .mockResolvedValueOnce({ refreshing: true })
      .mockResolvedValue({ refreshing: false });

    await clickRefreshArtwork();
    expect(mockedRefreshAll).toHaveBeenCalledTimes(1);
    expect(refreshButtonLabel()).toBe("Refreshing...");

    await advance(2500); // still refreshing
    expect(completeNotifications()).toBe(0);
    const showsFetches = mockedGetShows.mock.calls.length;

    await advance(2500); // finished
    expect(completeNotifications()).toBe(1);
    expect(refreshButtonLabel()).toBe("Refresh artwork");
    expect(mockedGetShows.mock.calls.length).toBeGreaterThan(showsFetches);

    // Polling stops once the job has ended.
    const polls = mockedGetStatus.mock.calls.length;
    await advance(60_000);
    expect(mockedGetStatus).toHaveBeenCalledTimes(polls);
  });

  it("keeps polling through a transient status-check error", async () => {
    await renderPage();
    mockedGetStatus
      .mockRejectedValueOnce(new Error("network blip"))
      .mockResolvedValueOnce({ refreshing: true })
      .mockResolvedValue({ refreshing: false });

    await clickRefreshArtwork();
    await advance(2500); // error, retried after a short delay
    await advance(2500);
    await advance(2500);
    await advance(2500);

    expect(completeNotifications()).toBe(1);
    expect(refreshButtonLabel()).toBe("Refresh artwork");
  });

  it("picks up a refresh that was already running when the page opened", async () => {
    mockedGetStatus.mockResolvedValueOnce({ refreshing: true }).mockResolvedValue({
      refreshing: false,
    });
    await renderPage();
    expect(refreshButtonLabel()).toBe("Refreshing...");

    // Found running at load: tracked at the slow cadence until it ends.
    await advance(30_000);
    expect(completeNotifications()).toBe(1);
    expect(mockedRefreshAll).not.toHaveBeenCalled();
  });
});
