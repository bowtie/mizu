import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// ── Module mocks ──
// The page reads and writes through the real query hooks; only the HTTP
// layer (api.ts), the notification store, routing and sounds are mocked.
// TrackingRuleCard is replaced with a plain button so a tracking-default
// change is one click (the real card is a styled Switch that is awkward to
// drive from happy-dom).

const { addNotificationMock } = vi.hoisted(() => ({ addNotificationMock: vi.fn() }));

const SETTINGS = {
  sonarr_url: "http://localhost:8989",
  sonarr_api_key_masked: "••••",
  sonarr_api_key_set: true,
  default_filler_action: "unmonitor",
  default_mixed_canon_action: "monitor",
  default_anime_canon_action: "monitor",
  sync_interval: "6h",
  sonarr_add_root_folder: "/tv",
  sonarr_add_quality_profile_id: 1,
  sonarr_add_monitor_mode: "all",
  sonarr_add_season_folder: true,
  afl_show_cache_refreshed_at: null,
  afl_show_cache_count: 0,
  artwork_default_source: "sonarr",
  ntfy_enabled: false,
  ntfy_server_url: "",
  ntfy_topic: "",
  ntfy_token_set: false,
  ui_sounds_enabled: true,
};

vi.mock("../lib/api", () => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  getArtworkProviders: vi.fn().mockResolvedValue({ providers: [] }),
  getAuthStatus: vi.fn().mockResolvedValue({
    configured: true,
    authenticated: true,
    mode: "password",
  }),
  refreshAflCache: vi.fn(),
  getAflCacheRefreshStatus: vi.fn(),
  refreshAllArtwork: vi.fn(),
  testNtfyConnection: vi.fn(),
  testSonarrConnection: vi.fn(),
  exportConfig: vi.fn(),
  importConfig: vi.fn(),
  changePassword: vi.fn(),
  getAccessSettings: vi.fn().mockResolvedValue({
    mode: "password",
    trusted_proxies: "",
    client_address: "127.0.0.1",
    via_trusted_proxy: true,
    forwarded: false,
    host: "localhost",
    allowed_hosts: "",
    password_set: true,
  }),
  updateAccessSettings: vi.fn(),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => addNotificationMock,
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock("../lib/sounds", () => ({
  applySoundsEnabled: vi.fn(),
}));

vi.mock("../components/TrackingRuleCard", async () => {
  const { createElement } = await import("react");
  return {
    TrackingRuleCard: ({
      label,
      value,
      onChange,
    }: {
      label: string;
      value: boolean;
      onChange: (v: boolean) => void;
    }) =>
      createElement(
        "button",
        { type: "button", "data-rule": label, onClick: () => onChange(!value) },
        label,
      ),
  };
});

import { getSettings, updateSettings, refreshAflCache, getAflCacheRefreshStatus } from "../lib/api";
import { SettingsPage } from "./SettingsPage";

const mockedGetSettings = vi.mocked(getSettings);
const updateSettingsMock = vi.mocked(updateSettings);
const mockedRefreshAflCache = vi.mocked(refreshAflCache);
const mockedAflStatus = vi.mocked(getAflCacheRefreshStatus);

let container: HTMLDivElement;
let root: Root;
let queryClient: QueryClient;

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  addNotificationMock.mockClear();
  mockedGetSettings.mockReset();
  mockedGetSettings.mockResolvedValue(SETTINGS as never);
  updateSettingsMock.mockReset();
  updateSettingsMock.mockResolvedValue({ status: "settings updated" });
  mockedAflStatus.mockReset();
  mockedAflStatus.mockResolvedValue({ refreshing: false });
  mockedRefreshAflCache.mockReset();
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  queryClient.clear();
});

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
      createElement(QueryClientProvider, { client: queryClient }, createElement(SettingsPage)),
    );
  });
  await flush();
}

function findButton(label: string): HTMLButtonElement | undefined {
  return Array.from(container.querySelectorAll("button")).find((b) =>
    b.textContent?.includes(label),
  );
}

describe("SettingsPage", () => {
  it("renders the settings form once settings load (no skeleton)", async () => {
    await renderPage();

    expect(container.textContent).toContain("Settings");
    expect(container.textContent).toContain("Sonarr URL");
    // The skeleton has no Replace button; a stored Sonarr key shows masked
    // with one, which confirms the loaded form rendered.
    expect(findButton("Replace")).toBeDefined();
    // Nothing edited yet, so the save bar stays hidden.
    expect(findButton("Save settings")).toBeUndefined();
  });

  it("shows the save bar only once a default rule changes, then persists via updateSettings", async () => {
    await renderPage();
    expect(updateSettingsMock).not.toHaveBeenCalled();

    // Toggle the Filler default (unmonitor -> monitor).
    const filler = container.querySelector<HTMLButtonElement>('button[data-rule="Filler"]');
    expect(filler).not.toBeNull();
    await act(async () => {
      filler!.click();
    });
    expect(container.textContent).toContain("You have unsaved changes");

    await act(async () => {
      findButton("Save settings")?.click();
    });
    await flush(3);

    expect(updateSettingsMock).toHaveBeenCalledTimes(1);
    expect(updateSettingsMock).toHaveBeenCalledWith(
      expect.objectContaining({ default_filler_action: "monitor" }),
    );
    expect(addNotificationMock).toHaveBeenCalledWith("Settings saved", "success");
    // The stored settings are re-read after saving.
    expect(mockedGetSettings.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("Discard reverts the edit and hides the save bar without saving", async () => {
    await renderPage();

    const filler = container.querySelector<HTMLButtonElement>('button[data-rule="Filler"]');
    await act(async () => {
      filler!.click();
    });
    expect(findButton("Discard")).toBeDefined();

    await act(async () => {
      findButton("Discard")?.click();
    });
    expect(findButton("Save settings")).toBeUndefined();
    expect(updateSettingsMock).not.toHaveBeenCalled();
  });

  it("Replace opens an empty key input and Cancel restores the masked display", async () => {
    await renderPage();

    await act(async () => {
      findButton("Replace")?.click();
    });
    expect(findButton("Cancel")).toBeDefined();
    expect(findButton("Replace")).toBeUndefined();

    await act(async () => {
      findButton("Cancel")?.click();
    });
    expect(findButton("Replace")).toBeDefined();
    expect(findButton("Save settings")).toBeUndefined();
  });

  it("registers a beforeunload warning when the form becomes dirty", async () => {
    await renderPage();

    const addSpy = vi.spyOn(window, "addEventListener");
    const filler = container.querySelector<HTMLButtonElement>('button[data-rule="Filler"]');
    await act(async () => {
      filler!.click();
    });

    expect(addSpy).toHaveBeenCalledWith("beforeunload", expect.any(Function));
    addSpy.mockRestore();
  });

  it("refreshes the AFL cache and reports when the background job ends", async () => {
    await renderPage();
    mockedRefreshAflCache.mockResolvedValue({ status: "refreshing" });
    // First poll still running, then done.
    mockedAflStatus.mockResolvedValueOnce({ refreshing: true }).mockResolvedValue({
      refreshing: false,
    });

    await act(async () => {
      findButton("Refresh")?.click();
    });
    await flush(2);
    expect(mockedRefreshAflCache).toHaveBeenCalledTimes(1);
    expect(container.innerHTML).toContain("Refreshing...");

    // Polls every 2s until the job reports it has finished.
    for (let i = 0; i < 3; i++) {
      await act(async () => {
        await new Promise((r) => setTimeout(r, 2100));
      });
    }
    await flush();

    expect(addNotificationMock).toHaveBeenCalledWith("AFL show list refreshed", "success");
    expect(container.innerHTML).not.toContain("Refreshing...");
  }, 15_000);

  it("keeps polling through a transient status-check error", async () => {
    await renderPage();
    mockedRefreshAflCache.mockResolvedValue({ status: "refreshing" });
    mockedAflStatus
      .mockRejectedValueOnce(new Error("network blip"))
      .mockResolvedValueOnce({ refreshing: true })
      .mockResolvedValue({ refreshing: false });

    await act(async () => {
      findButton("Refresh")?.click();
    });
    // Poll errors, retries after a short delay, then polls again until done.
    for (let i = 0; i < 4; i++) {
      await act(async () => {
        await new Promise((r) => setTimeout(r, 2100));
      });
    }
    await flush();

    expect(addNotificationMock).toHaveBeenCalledWith("AFL show list refreshed", "success");
    expect(container.innerHTML).not.toContain("Refreshing...");
  }, 20_000);

  it("picks up a refresh that was already running when the page opened", async () => {
    mockedAflStatus.mockResolvedValueOnce({ refreshing: true }).mockResolvedValue({
      refreshing: false,
    });
    await renderPage();
    expect(container.innerHTML).toContain("Refreshing...");

    await act(async () => {
      await new Promise((r) => setTimeout(r, 2100));
    });
    await flush();

    expect(addNotificationMock).toHaveBeenCalledWith("AFL show list refreshed", "success");
    expect(mockedRefreshAflCache).not.toHaveBeenCalled();
  }, 10_000);
});
