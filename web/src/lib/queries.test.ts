import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Show, Episode } from "./schemas";

vi.mock("./api", () => ({
  getShow: vi.fn(),
  getShows: vi.fn(),
  getEpisodes: vi.fn(),
  getSettings: vi.fn(),
  getSonarrSeries: vi.fn(),
  getArtworkProviders: vi.fn(),
  getSyncLogs: vi.fn(),
  updateShow: vi.fn(),
  deleteShow: vi.fn(),
  updateSettings: vi.fn(),
  triggerSync: vi.fn(),
  setEpisodeMonitor: vi.fn(),
  getAuthStatus: vi.fn(),
}));

import { getShow, getEpisodes, setEpisodeMonitor } from "./api";
import { queryKeys, useEpisodes, useSetEpisodeMonitor, useShow } from "./queries";

const mockedGetShow = vi.mocked(getShow);
const mockedGetEpisodes = vi.mocked(getEpisodes);
const mockedSetEpisodeMonitor = vi.mocked(setEpisodeMonitor);

function makeShow(overrides: Partial<Show> = {}): Show {
  return {
    id: 5,
    sonarr_series_id: 101,
    sonarr_title: "Frieren",
    sync_watermark: 0,
    enabled: true,
    filler_action: "unmonitor",
    mixed_action: "monitor",
    anime_canon_action: "monitor",
    watermark_mode: "auto",
    ...overrides,
  };
}

let container: HTMLDivElement;
let root: Root;
let qc: QueryClient;

beforeEach(() => {
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  vi.clearAllMocks();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

async function flush(rounds = 4) {
  for (let i = 0; i < rounds; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function render(child: () => null) {
  await act(async () => {
    root.render(createElement(QueryClientProvider, { client: qc }, createElement(child)));
  });
}

describe("queryKeys", () => {
  it("addresses the same show whether the id is a number or a URL string", () => {
    expect(queryKeys.show(5)).toEqual(queryKeys.show("5"));
    expect(queryKeys.episodes(5)).toEqual(queryKeys.episodes("5"));
  });
});

describe("useShow", () => {
  it("shows the library's copy of the show while its own query loads", async () => {
    qc.setQueryData(queryKeys.shows, [makeShow({ sonarr_title: "From library" })]);
    let resolveShow!: (s: Show) => void;
    mockedGetShow.mockReturnValue(new Promise((r) => (resolveShow = r)));

    let seen: { title?: string; placeholder: boolean }[] = [];
    await render(() => {
      const q = useShow("5");
      seen.push({ title: q.data?.sonarr_title, placeholder: q.isPlaceholderData });
      return null;
    });
    expect(seen.at(-1)).toEqual({ title: "From library", placeholder: true });

    seen = [];
    await act(async () => resolveShow(makeShow({ sonarr_title: "Fresh" })));
    await flush();
    expect(seen.at(-1)).toEqual({ title: "Fresh", placeholder: false });
  });
});

describe("useSetEpisodeMonitor", () => {
  it("patches only the toggled episode in the cached list, without refetching", async () => {
    const episodes: Episode[] = [
      { sonarr_episode_id: 1, monitored: false },
      { sonarr_episode_id: 2, monitored: false },
    ];
    mockedGetEpisodes.mockResolvedValue(episodes);
    mockedSetEpisodeMonitor.mockResolvedValue({ monitored: true });

    let toggle!: () => Promise<unknown>;
    await render(() => {
      useEpisodes(5);
      const m = useSetEpisodeMonitor();
      toggle = () => m.mutateAsync({ showId: "5", epId: 2, monitored: true });
      return null;
    });
    await flush();
    expect(mockedGetEpisodes).toHaveBeenCalledTimes(1);

    await act(async () => {
      await toggle();
    });
    await flush();

    expect(mockedSetEpisodeMonitor).toHaveBeenCalledWith("5", 2, true);
    expect(qc.getQueryData(queryKeys.episodes(5))).toEqual([
      { sonarr_episode_id: 1, monitored: false },
      { sonarr_episode_id: 2, monitored: true },
    ]);
    expect(mockedGetEpisodes).toHaveBeenCalledTimes(1);
  });
});
