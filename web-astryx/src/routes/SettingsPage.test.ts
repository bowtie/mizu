import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";

// ── Module mocks ──
// The page talks to the backend through the settings mutation and test
// endpoints, to the notification store, to react-query (useSettings/
// useUpdateSettings/useQueryClient), and to sounds. Mock those so the tests
// drive the form with controlled data. TrackingRuleCard is replaced with a
// plain button so a tracking-default change is one click (the real card is a
// styled Switch that is awkward to drive from happy-dom).

const { addNotificationMock, updateSettingsMock, queryClient } = vi.hoisted(() => ({
  addNotificationMock: vi.fn(),
  updateSettingsMock: vi.fn(),
  // useQueryClient must return a *stable* object: loadSettings is a useCallback
  // keyed on queryClient, so a fresh object per render would re-run the
  // settings fetch (and reset the form) every render and loop.
  queryClient: { fetchQuery: vi.fn() },
}));

const EMPTY_PROVIDERS: never[] = [];

// Stable settings fixture. useSettings returns this exact object, so the
// store-settings effect dep never changes between renders.
const SETTINGS = {
  sonarr_url: "http://localhost:8989",
  sonarr_api_key: "secret",
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
} as const;

vi.mock("../lib/api", () => ({
  getSettings: vi.fn(),
  refreshAflCache: vi.fn(),
  refreshAllArtwork: vi.fn(),
  testNtfyConnection: vi.fn(),
  testSonarrConnection: vi.fn(),
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../lib/queries", () => ({
  useSettings: () => ({ data: SETTINGS, isPending: false, isFetched: true }),
  useArtworkProviders: () => ({ data: EMPTY_PROVIDERS, isPending: false, isFetched: true }),
  useUpdateSettings: () => ({ mutateAsync: updateSettingsMock }),
}));

vi.mock("../lib/store", () => ({
  useNotificationStore: () => addNotificationMock,
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => queryClient,
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

import { SettingsPage } from "./SettingsPage";

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  addNotificationMock.mockClear();
  updateSettingsMock.mockReset();
  updateSettingsMock.mockResolvedValue(SETTINGS);
  (queryClient.fetchQuery as ReturnType<typeof vi.fn>).mockResolvedValue(SETTINGS);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
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
    root.render(createElement(SettingsPage));
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
    // The skeleton heading is a bare div, not the real <h1> "Settings" — the
    // presence of a real Save button confirms the loaded form rendered.
    expect(findButton("Save settings")).toBeDefined();
  });

  it("keeps Save disabled until a default rule changes, then persists via updateSettings", async () => {
    await renderPage();

    // Nothing dirty yet — clicking Save must not persist anything.
    await act(async () => {
      findButton("Save settings")?.click();
    });
    expect(updateSettingsMock).not.toHaveBeenCalled();

    // Toggle the Filler default (unmonitor -> monitor).
    const filler = container.querySelector<HTMLButtonElement>('button[data-rule="Filler"]');
    expect(filler).not.toBeNull();
    await act(async () => {
      filler!.click();
    });
    expect(container.textContent).toContain("Unsaved changes");

    await act(async () => {
      findButton("Save settings")?.click();
    });
    await flush(3);

    expect(updateSettingsMock).toHaveBeenCalledTimes(1);
    expect(updateSettingsMock).toHaveBeenCalledWith(
      expect.objectContaining({ default_filler_action: "monitor" }),
    );
    expect(addNotificationMock).toHaveBeenCalledWith("Settings saved", "success");
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
});
