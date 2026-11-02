import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";

// ── Module mocks ──
// The astryx Dialog is a native <dialog> element that needs showModal();
// happy-dom doesn't drive it, so stub Dialog/DialogHeader to render children
// inline. The candidate fetch goes through getArtworkCandidates. TabList/Tab
// render as real <nav>/<button> elements, so the tests can click them.
const { getArtworkCandidatesMock } = vi.hoisted(() => ({
  getArtworkCandidatesMock: vi.fn(),
}));

vi.mock("../lib/api", () => ({
  getArtworkCandidates: getArtworkCandidatesMock,
}));

vi.mock("@astryxdesign/core/Dialog", () => ({
  Dialog: ({ isOpen, children }: { isOpen: boolean; children: ReactNode }) =>
    isOpen ? createElement("div", { "data-testid": "dialog" }, children) : null,
  DialogHeader: ({ title }: { title: string }) =>
    createElement("div", { "data-testid": "dialog-header" }, title),
}));

import { ArtworkPickerDialog, type ArtworkPickerSource } from "./ArtworkPickerDialog";
import type { ArtworkOptions } from "../lib/types";

const SOURCES: ArtworkPickerSource[] = [
  { id: "fanart", label: "Fanart.tv" },
  { id: "tmdb", label: "TMDB" },
];

const OPTIONS: ArtworkOptions = {
  posters: [
    { url: "https://example.com/p1.jpg", thumb_url: "https://example.com/p1_t.jpg", likes: 5 },
  ],
  banners: [{ url: "https://example.com/b1.jpg" }],
  logos: [{ url: "https://example.com/logo.svg" }],
};

const TMDB_OPTIONS: ArtworkOptions = {
  posters: [{ url: "https://example.com/tmdb-p1.jpg" }],
  banners: [],
  logos: [],
};

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  getArtworkCandidatesMock.mockReset();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

async function flush() {
  for (let i = 0; i < 6; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function renderPicker(
  onSelect: (source: string, slot: string, url: string) => void,
  initialSource = "fanart",
  currentArtwork?: Partial<Record<string, string>>,
  extra: Record<string, unknown> = {},
) {
  await act(async () => {
    root.render(
      createElement(ArtworkPickerDialog, {
        isOpen: true,
        onOpenChange: () => {},
        showId: 1,
        sources: SOURCES,
        initialSource,
        currentArtwork,
        onSelect,
        artworkSource: "",
        artworkSourceOptions: [
          { value: "", label: "Use global default" },
          { value: "fanart", label: "Fanart.tv" },
        ],
        refreshing: false,
        onArtworkSourceChange: () => {},
        onRefresh: () => {},
        customSlots: [],
        onUploadCustom: () => {},
        onRemoveCustom: () => {},
        ...extra,
      }),
    );
  });
  await flush();
}

/** Only the active slot's thumbnails are rendered at a time. */
function pickTiles(): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(".artwork-picker-item"));
}

function tab(value: string): HTMLButtonElement | null {
  return container.querySelector<HTMLButtonElement>(`button[data-tab-value="${value}"]`);
}

/** Source switcher segment (a role=radio button), found by its visible label. */
function sourceSegment(label: string): HTMLButtonElement | undefined {
  return Array.from(container.querySelectorAll<HTMLButtonElement>('button[role="radio"]')).find(
    (b) => b.textContent?.includes(label),
  );
}

describe("ArtworkPickerDialog", () => {
  it("fetches the initial source and emits a selection with source and slot", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    const onSelect = vi.fn();
    await renderPicker(onSelect);

    expect(getArtworkCandidatesMock).toHaveBeenCalledWith(1, "fanart");

    // Poster is the default slot, so only its thumbnails are shown.
    const tiles = pickTiles();
    expect(tiles.length).toBe(1);

    await act(async () => {
      tiles[0].click();
    });
    expect(onSelect).toHaveBeenCalledWith("fanart", "poster", "https://example.com/p1.jpg");
  });

  it("switches slot tabs to reveal banners and logos with their own aspect slots", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    const onSelect = vi.fn();
    await renderPicker(onSelect);

    await act(async () => {
      tab("banner")?.click();
    });
    expect(pickTiles()).toHaveLength(1);
    expect(pickTiles()[0].dataset.slot).toBe("banner");

    await act(async () => {
      pickTiles()[0].click();
    });
    expect(onSelect).toHaveBeenCalledWith("fanart", "banner", "https://example.com/b1.jpg");

    await act(async () => {
      tab("logo")?.click();
    });
    expect(pickTiles()[0].dataset.slot).toBe("logo");
  });

  it("switches source tabs and caches each source's results", async () => {
    getArtworkCandidatesMock.mockResolvedValueOnce(OPTIONS).mockResolvedValueOnce(TMDB_OPTIONS);
    await renderPicker(() => {});

    expect(getArtworkCandidatesMock).toHaveBeenCalledTimes(1);
    expect(getArtworkCandidatesMock).toHaveBeenCalledWith(1, "fanart");

    // Switch to TMDB: a second fetch fires.
    await act(async () => {
      sourceSegment("TMDB")?.click();
    });
    await flush();
    expect(getArtworkCandidatesMock).toHaveBeenCalledTimes(2);
    expect(getArtworkCandidatesMock).toHaveBeenCalledWith(1, "tmdb");
    expect(pickTiles()[0].querySelector("img")?.src).toContain("tmdb-p1.jpg");

    // Switch back and forth: both sources are now cached, no new fetches.
    await act(async () => {
      sourceSegment("Fanart")?.click();
    });
    await act(async () => {
      sourceSegment("TMDB")?.click();
    });
    expect(getArtworkCandidatesMock).toHaveBeenCalledTimes(2);
  });

  it("uses the thumbnail URL and documents the slot in the image alt text", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    await renderPicker(() => {});

    const img = container.querySelector<HTMLImageElement>("img");
    expect(img).not.toBeNull();
    expect(img!.src).toContain("p1_t.jpg");
    expect(img!.alt).toBe("Posters 1");
  });

  it("shows an error message when the candidates fetch fails", async () => {
    getArtworkCandidatesMock.mockRejectedValue(new Error("fanart is not configured"));
    await renderPicker(() => {});

    expect(container.textContent).toContain("fanart is not configured");
    expect(pickTiles().length).toBe(0);
  });

  it("shows an empty message when the source returns no candidates", async () => {
    getArtworkCandidatesMock.mockResolvedValue({ posters: [], banners: [], logos: [] });
    await renderPicker(() => {});

    expect(container.textContent).toContain("has no images");
    expect(pickTiles().length).toBe(0);
  });

  it("rings the tile matching the show's currently-set artwork", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    await renderPicker(() => {}, "fanart", { poster: "https://example.com/p1.jpg" });

    const tiles = pickTiles();
    expect(tiles).toHaveLength(1);
    expect(tiles[0].classList.contains("is-selected")).toBe(true);
    expect(tiles[0].dataset.selected).toBe("true");
    expect(tiles[0].querySelector(".artwork-picker-check")).not.toBeNull();
  });

  it("does not ring tiles when the current artwork is from another source/slot", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    // Banner is set on the show; poster tab is active, so no poster matches.
    await renderPicker(() => {}, "fanart", { banner: "https://example.com/b1.jpg" });
    expect(pickTiles()[0].classList.contains("is-selected")).toBe(false);

    // Switching to the banner tab rings the matching banner.
    await act(async () => {
      tab("banner")?.click();
    });
    expect(pickTiles()[0].classList.contains("is-selected")).toBe(true);
  });

  it("renders without a ring when the dialog has no current artwork prop", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    await renderPicker(() => {});
    expect(pickTiles()[0].classList.contains("is-selected")).toBe(false);
    expect(pickTiles()[0].dataset.selected).toBeUndefined();
  });
});

describe("ArtworkPickerDialog source controls", () => {
  it("refreshes the show's artwork from the dialog", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    const onRefresh = vi.fn();
    await renderPicker(vi.fn(), "fanart", undefined, { onRefresh });

    const refresh = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Refresh"),
    );
    expect(refresh).toBeDefined();
    await act(async () => {
      refresh!.click();
    });
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it("shows Refresh as busy while a refresh is running", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    await renderPicker(vi.fn(), "fanart", undefined, { refreshing: true });

    const refresh = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Refreshing"),
    );
    expect(refresh?.getAttribute("aria-busy")).toBe("true");
  });

  it("points at Settings when no source offers images to browse", async () => {
    await renderPicker(vi.fn(), "", undefined, { sources: [] });

    expect(container.textContent).toContain("Add a TMDB or Fanart key");
    expect(getArtworkCandidatesMock).not.toHaveBeenCalled();
  });

  it("closes only from Done, not after picking an image", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    const onOpenChange = vi.fn();
    const onSelect = vi.fn();
    await renderPicker(onSelect, "fanart", undefined, { onOpenChange });

    await act(async () => {
      pickTiles()[0].click();
    });
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onOpenChange).not.toHaveBeenCalled();

    const done = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Done"),
    );
    await act(async () => {
      done!.click();
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("ArtworkPickerDialog custom uploads", () => {
  function chooseFile(file: File) {
    const input = container.querySelector<HTMLInputElement>('[data-testid="artwork-upload-input"]');
    expect(input).not.toBeNull();
    Object.defineProperty(input!, "files", { value: [file], configurable: true });
    return act(async () => {
      input!.dispatchEvent(new Event("change", { bubbles: true }));
    });
  }

  it("sends a chosen file for the slot being viewed", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    const onUploadCustom = vi.fn();
    await renderPicker(vi.fn(), "fanart", undefined, { onUploadCustom });

    // The poster tab is active by default; switch to banners first.
    await act(async () => {
      tab("banner")?.click();
    });
    const file = new File(["x"], "mine.png", { type: "image/png" });
    await chooseFile(file);

    expect(onUploadCustom).toHaveBeenCalledWith("banner", file);
  });

  it("limits the file picker to the formats the server accepts", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    await renderPicker(vi.fn());

    const input = container.querySelector<HTMLInputElement>('[data-testid="artwork-upload-input"]');
    expect(input?.accept).toBe("image/jpeg,image/png,image/gif,image/webp");
  });

  it("shows the upload badge and Remove only for a slot that holds an upload", async () => {
    getArtworkCandidatesMock.mockResolvedValue(OPTIONS);
    const onRemoveCustom = vi.fn();
    await renderPicker(vi.fn(), "fanart", undefined, { customSlots: ["poster"], onRemoveCustom });

    expect(container.textContent).toContain("Your image is in use");
    const remove = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Remove"),
    );
    expect(remove).toBeDefined();
    await act(async () => {
      remove!.click();
    });
    expect(onRemoveCustom).toHaveBeenCalledWith("poster");

    // Another slot has no upload, so the badge and Remove go away.
    await act(async () => {
      tab("banner")?.click();
    });
    expect(container.textContent).not.toContain("Your image is in use");
  });

  it("still offers uploads when no source has images to browse", async () => {
    await renderPicker(vi.fn(), "", undefined, { sources: [] });

    expect(getArtworkCandidatesMock).not.toHaveBeenCalled();
    expect(container.querySelector('[data-testid="artwork-upload-input"]')).not.toBeNull();
    expect(container.textContent).toContain("Upload poster");
  });
});
