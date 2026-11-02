import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// ── Module mocks ──
// The page reads through the real query hooks; only the HTTP layer and the
// notification store are mocked. The lazy AddAnimeDialog chunk is replaced with a null stub: the
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

import { getShows } from "../lib/api";
import { ShowsPage } from "./ShowsPage";

const mockedGetShows = vi.mocked(getShows);

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

describe("ShowsPage", () => {
  it("renders the library from the shows query", async () => {
    await renderPage();
    expect(container.textContent).toContain("Naruto");
    expect(container.textContent).toContain("1 tracked");
  });
});
