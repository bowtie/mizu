import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MemoryRouter, Routes, Route } from "react-router-dom";

// ── Module mocks ──
// The page talks to the backend through getShow/getEpisodes/updateShow/
// setEpisodeMonitor and friends, to the notification store, and to several
// react-query hooks. Mock all of them so the tests drive rendering with
// controlled responses. The heavy child components (virtualized list, rule
// cards, lazy AlertDialog) are stubbed out.

const { addNotificationMock } = vi.hoisted(() => ({ addNotificationMock: vi.fn() }));

// NOTE: the data values must be module-level constants. The page feeds query
// data into memo/effect deps; a fresh array per render would change the dep
// every render and loop.
const EMPTY_SHOWS: never[] = [];
const EMPTY_SERIES: never[] = [];
const EMPTY_PROVIDERS: never[] = [];

vi.mock("../lib/api", () => ({
  getShow: vi.fn(),
  getEpisodes: vi.fn(),
  getArtworkBannerUrl: vi.fn().mockReturnValue("/api/artwork/banner/1"),
  getCachedShowDetail: vi.fn().mockReturnValue(null),
  clearCachedShowDetail: vi.fn(),
  refreshArtwork: vi.fn(),
  setArtworkSource: vi.fn(),
  selectArtwork: vi.fn(),
  getArtworkCandidates: vi.fn(),
  setEpisodeMonitor: vi.fn(),
  updateShow: vi.fn(),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../lib/queries", () => ({
  useShows: () => ({ data: EMPTY_SHOWS, isPending: false, isFetched: true }),
  useArtworkProviders: () => ({ data: EMPTY_PROVIDERS, isPending: false, isFetched: true }),
  useDeleteShow: () => ({ mutateAsync: vi.fn().mockResolvedValue(undefined) }),
  useSonarrSeries: () => ({ data: EMPTY_SERIES, isPending: false, isFetched: true }),
  useSettings: () => ({ data: null, isPending: false, isFetched: true }),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => addNotificationMock,
}));

vi.mock("../components/VirtualizedEpisodeList", () => ({
  VirtualizedEpisodeList: () => null,
}));

vi.mock("../components/TrackingRuleCard", () => ({
  TrackingRuleCard: () => null,
}));

// Stub the picker dialog: render a button that emits a poster pick when the
// dialog is open, so the page-level test can drive handleArtworkPick without
// the astryx native <dialog> element (unsupported under happy-dom).
vi.mock("../components/ArtworkPickerDialog", () => ({
  ArtworkPickerDialog: (props: {
    isOpen: boolean;
    initialSource?: string;
    currentArtwork?: { poster?: string; banner?: string; logo?: string };
    onSelect: (source: string, slot: string, url: string) => void;
  }) => {
    if (!props.isOpen) return null;
    return createElement(
      "button",
      {
        "data-testid": "picker-pick",
        "data-current-poster": props.currentArtwork?.poster ?? "",
        onClick: () =>
          props.onSelect(
            props.initialSource ?? "fanart",
            "poster",
            "https://example.com/picked.jpg",
          ),
      },
      "Pick",
    );
  },
}));

import { getShow, getEpisodes, updateShow, setEpisodeMonitor, selectArtwork } from "../lib/api";
import type { Show, Episode } from "../lib/types";
import { ShowDetailPage } from "./ShowDetailPage";

const mockedGetShow = vi.mocked(getShow);
const mockedGetEpisodes = vi.mocked(getEpisodes);
const mockedUpdateShow = vi.mocked(updateShow);
const mockedSetEpisodeMonitor = vi.mocked(setEpisodeMonitor);
const mockedSelectArtwork = vi.mocked(selectArtwork);

const SHOW: Show = {
  id: 1,
  sonarr_series_id: 7,
  sonarr_title: "Naruto",
  overview: "A young ninja seeks recognition.",
  filler_action: "unmonitor",
  mixed_action: "monitor",
  anime_canon_action: "monitor",
  watermark_mode: "auto",
  sync_watermark: 0,
  enabled: true,
};

const EPISODE: Episode = {
  id: 1,
  sonarr_episode_id: 101,
  episode_num: 1,
  season_number: 1,
  sonarr_episode_number: 1,
  absolute_episode_number: 1,
  title: "Enter: Naruto",
  type: "filler",
  monitored: false,
};

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  mockedGetShow.mockReset();
  mockedGetEpisodes.mockReset();
  mockedUpdateShow.mockReset();
  mockedSetEpisodeMonitor.mockReset();
  mockedSelectArtwork.mockReset();
  addNotificationMock.mockClear();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

// Real timers: these pages have no polling loop. `flush` drains the async
// effects (getShow/getEpisodes promises) and the deferred episode-list value.
async function flush(rounds = 6) {
  for (let i = 0; i < rounds; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function renderPage() {
  await act(async () => {
    root.render(
      createElement(
        MemoryRouter,
        { initialEntries: ["/shows/1"] },
        createElement(
          Routes,
          null,
          createElement(Route, { path: "/shows/:id", element: createElement(ShowDetailPage) }),
        ),
      ),
    );
  });
  await flush();
}

function findButton(label: string): HTMLButtonElement | undefined {
  return Array.from(container.querySelectorAll("button")).find((b) =>
    b.textContent?.includes(label),
  );
}

describe("ShowDetailPage", () => {
  it("renders the show title, overview, and episodes once data loads", async () => {
    mockedGetShow.mockResolvedValue(SHOW);
    mockedGetEpisodes.mockResolvedValue([EPISODE]);
    await renderPage();

    expect(container.textContent).toContain("Naruto");
    expect(container.textContent).toContain("A young ninja seeks recognition.");
    // Episode list renders the episode title and its type badge.
    expect(container.textContent).toContain("Enter: Naruto");
    expect(container.textContent).toContain("Filler");
  });

  it("keeps Save disabled until a tracking rule changes, then persists via updateShow", async () => {
    mockedGetShow.mockResolvedValue(SHOW);
    mockedGetEpisodes.mockResolvedValue([EPISODE]);
    await renderPage();

    // Nothing is dirty yet — clicking Save must not persist anything.
    await act(async () => {
      findButton("Save")?.click();
    });
    expect(mockedUpdateShow).not.toHaveBeenCalled();

    // Toggle the watermark mode Auto -> Manual. This is a real change to the
    // show's tracking rules, so the Save button becomes enabled and the
    // "Unsaved changes" badge appears.
    const autoToggle = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Using Sonarr progress"]',
    );
    expect(autoToggle).not.toBeNull();
    await act(async () => {
      autoToggle!.click();
    });
    expect(container.textContent).toContain("Unsaved changes");

    await act(async () => {
      findButton("Save")?.click();
    });
    await flush(3);

    expect(mockedUpdateShow).toHaveBeenCalledTimes(1);
    expect(mockedUpdateShow).toHaveBeenCalledWith(
      "1",
      expect.objectContaining({ watermark_mode: "manual" }),
    );
    expect(addNotificationMock).toHaveBeenCalledWith(
      expect.stringContaining("Saved tracking rules"),
      "success",
    );
  });

  it("opens the picker for a multi-image source and persists a partial pick", async () => {
    mockedGetShow.mockResolvedValue({ ...SHOW, artwork_source: "fanart" });
    mockedGetEpisodes.mockResolvedValue([EPISODE]);
    mockedSelectArtwork.mockResolvedValue({ source: "fanart", found: true });
    await renderPage();

    // The "Choose" button only appears when the resolved source supports
    // candidate browsing (fanart/tmdb).
    const choose = findButton("Choose");
    expect(choose).toBeDefined();
    await act(async () => {
      choose!.click();
    });
    await flush(2);

    const pick = container.querySelector<HTMLButtonElement>('[data-testid="picker-pick"]');
    expect(pick).not.toBeNull();
    await act(async () => {
      pick!.click();
    });
    await flush(3);

    // Only the picked slot is sent; the other slots stay empty so the
    // backend preserves their existing artwork.
    expect(mockedSelectArtwork).toHaveBeenCalledWith("1", {
      source: "fanart",
      poster_url: "https://example.com/picked.jpg",
      banner_url: "",
      logo_url: "",
    });
    expect(addNotificationMock).toHaveBeenCalledWith(expect.stringContaining("updated"), "success");
  });

  it("passes the show's current artwork URLs to the picker", async () => {
    mockedGetShow.mockResolvedValue({
      ...SHOW,
      artwork_source: "fanart",
      artwork_poster_url: "https://example.com/current-poster.jpg",
    });
    mockedGetEpisodes.mockResolvedValue([EPISODE]);
    await renderPage();

    await act(async () => {
      findButton("Choose")!.click();
    });
    await flush(2);

    const pick = container.querySelector<HTMLButtonElement>('[data-testid="picker-pick"]');
    expect(pick?.dataset.currentPoster).toBe("https://example.com/current-poster.jpg");
  });

  it("hides the picker for a source without candidate browsing", async () => {
    mockedGetShow.mockResolvedValue({ ...SHOW, artwork_source: "kitsu" });
    mockedGetEpisodes.mockResolvedValue([EPISODE]);
    await renderPage();

    expect(findButton("Choose")).toBeUndefined();
  });

  it("toggles an episode's monitor state via setEpisodeMonitor", async () => {
    mockedGetShow.mockResolvedValue(SHOW);
    mockedGetEpisodes.mockResolvedValue([EPISODE]);
    await renderPage();

    // astryx ToggleButton is icon-only, so its `label` becomes the accessible
    // name (the consumer aria-label is overridden by Button's ariaLabelProp).
    const toggle = container.querySelector<HTMLElement>('[aria-label="Monitor episode"]');
    expect(toggle).not.toBeNull();
    await act(async () => {
      toggle!.click();
    });
    await flush(2);

    expect(mockedSetEpisodeMonitor).toHaveBeenCalledWith("1", 101, true);
  });
});
