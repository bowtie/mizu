import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// The astryx Dialog needs showModal(), which happy-dom doesn't drive, so it
// renders inline here (as in ArtworkPickerDialog.test.ts).
const { getFillerFilesMock, deleteFillerFilesMock } = vi.hoisted(() => ({
  getFillerFilesMock: vi.fn(),
  deleteFillerFilesMock: vi.fn(),
}));

vi.mock("../lib/api", () => ({
  getFillerFiles: getFillerFilesMock,
  deleteFillerFiles: deleteFillerFilesMock,
  errorMessage: (err: unknown) => String(err),
}));

vi.mock("@astryxdesign/core/Dialog", () => ({
  Dialog: ({ isOpen, children }: { isOpen: boolean; children: ReactNode }) =>
    isOpen ? createElement("div", { "data-testid": "dialog" }, children) : null,
  DialogHeader: ({ title }: { title: string }) => createElement("div", null, title),
}));

import { FillerFilesDialog, formatBytes } from "./FillerFilesDialog";

const FILES = [
  {
    episode_file_id: 11,
    path: "S01E01.mkv",
    size: 1 << 30,
    episodes: [{ sonarr_episode_id: 1, episode_num: 1, title: "Beach", type: "filler" }],
    monitored: false,
  },
  {
    episode_file_id: 14,
    path: "S01E05.mkv",
    size: 512 << 20,
    episodes: [{ sonarr_episode_id: 5, episode_num: 5, title: "Hot spring", type: "filler" }],
    monitored: true,
  },
];

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  getFillerFilesMock.mockResolvedValue({ files: FILES });
  deleteFillerFilesMock.mockResolvedValue({ deleted: 1, freed_bytes: 1 << 30 });
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

async function render(onOpenChange = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  await act(async () => {
    root.render(
      createElement(
        QueryClientProvider,
        { client },
        createElement(FillerFilesDialog, {
          isOpen: true,
          onOpenChange,
          showId: "3",
          isMobile: false,
        }),
      ),
    );
  });
  await flush();
  return onOpenChange;
}

const deleteButton = () =>
  [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("Delete"));

describe("FillerFilesDialog", () => {
  it("picks every file except ones monitored again", async () => {
    await render();
    const boxes = [...container.querySelectorAll<HTMLInputElement>("input[type=checkbox]")];
    expect(boxes.map((b) => b.checked)).toEqual([true, false]);
    expect(container.textContent).toContain("Monitored");
    expect(deleteButton()?.textContent).toContain("Delete 1 (1.0 GiB)");
  });

  it("deletes the picked files and closes", async () => {
    const onOpenChange = await render();
    await act(async () => deleteButton()?.click());
    await flush();
    expect(deleteFillerFilesMock).toHaveBeenCalledWith("3", [11]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("says so when there is nothing to delete", async () => {
    getFillerFilesMock.mockResolvedValue({ files: [] });
    await render();
    expect(container.textContent).toContain("No downloaded files hold only skipped episodes.");
    expect(deleteButton()).toBeUndefined();
    expect([...container.querySelectorAll("button")].map((b) => b.textContent)).toContain("Close");
  });
});

describe("formatBytes", () => {
  it("uses binary units", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1536)).toBe("1.5 KiB");
    expect(formatBytes(14 * 2 ** 30)).toBe("14.0 GiB");
  });
});
