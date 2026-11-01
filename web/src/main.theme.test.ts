import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from "vitest";
import { act, createElement } from "react";

// ── App-shell mocks ──
// The race lives in Root's handleSelect (main.tsx). We render the real Root
// through its module-level createRoot().render() call, stubbing only the
// design-system chrome and lazy routes — the theme selection logic under test
// stays real.

// React's createElement can't be referenced inside vi.mock factories (they're
// hoisted above imports), so stash it in a hoisted holder assigned in beforeAll.
const h = vi.hoisted(() => ({
  createElement: null as unknown as (
    type: string | ((props: unknown) => unknown),
    props?: Record<string, unknown> | null,
    ...children: unknown[]
  ) => unknown,
}));

const themeMocks = vi.hoisted(() => ({
  loadThemeDef: vi.fn<(id: string) => Promise<unknown>>(),
  loadThemeCSS: vi.fn<(id: string) => Promise<void>>(),
}));

vi.mock("./lib/themes", () => ({
  themeMetas: [
    { id: "flexoki", label: "Flexoki", description: "default", mode: "light" },
    { id: "tokyo-night", label: "Tokyo Night", description: "dark", mode: "dark" },
  ],
  defaultTheme: {
    id: "flexoki",
    label: "Flexoki",
    description: "default",
    theme: { id: "flexoki" },
  },
  loadThemeDef: themeMocks.loadThemeDef,
  loadThemeCSS: themeMocks.loadThemeCSS,
}));

vi.mock("./App", () => ({
  App: () => null,
}));

// main.tsx fires `void import("./routes/ShowsPage")` and `initSounds()` at
// module scope / on mount — stub both so the test stays isolated from
// production module side effects.
vi.mock("./routes/ShowsPage", () => ({
  ShowsPage: () => null,
}));

vi.mock("./lib/sounds", () => ({
  initSounds: () => undefined,
}));

// Theme renders a data attribute so the applied theme is observable in the DOM.
vi.mock("@astryxdesign/core/theme", () => ({
  Theme: ({
    theme,
    mode,
    children,
  }: {
    theme: { id?: string };
    mode?: string;
    children?: unknown;
  }) =>
    h.createElement(
      "div",
      { "data-theme-id": theme?.id ?? "none", "data-theme-mode": mode ?? "" },
      children,
    ),
}));

// ThemeMenu renders its options via the lazy DropdownMenu — stub it as plain
// buttons so the test can drive handleSelect through the real items wiring.
vi.mock("@astryxdesign/core/DropdownMenu", () => ({
  DropdownMenu: ({ items }: { items: Array<{ label: string; onClick: () => void }> }) =>
    h.createElement(
      "div",
      null,
      ...items.map((it) =>
        h.createElement(
          "button",
          { key: it.label, "data-theme-item": it.label, onClick: it.onClick },
          it.label,
        ),
      ),
    ),
}));

// The theme menu is passed to TopNav as endContent — render that.
vi.mock("@astryxdesign/core/TopNav", () => ({
  TopNav: ({ endContent }: { endContent?: unknown }) => h.createElement("nav", null, endContent),
  TopNavHeading: ({ children }: { children?: unknown }) => h.createElement("span", null, children),
  TopNavItem: ({ children }: { children?: unknown }) => h.createElement("span", null, children),
}));

// The real AppShell renders the topNav prop as its own region, separate from
// children — mirror that so the theme menu (passed as topNav's endContent)
// actually lands in the DOM.
vi.mock("@astryxdesign/core/AppShell", () => ({
  AppShell: ({ topNav, children }: { topNav?: unknown; children?: unknown }) =>
    h.createElement("div", null, topNav, children),
}));

vi.mock("@astryxdesign/core/NavIcon", () => ({
  NavIcon: ({ icon }: { icon?: unknown }) => h.createElement("span", null, icon),
}));

vi.mock("@tanstack/react-query-devtools", () => ({
  ReactQueryDevtools: () => null,
}));

describe("theme selection supersede guard", () => {
  beforeAll(() => {
    h.createElement = createElement as unknown as (typeof h)["createElement"];
  });

  beforeEach(() => {
    vi.useFakeTimers();
    themeMocks.loadThemeDef.mockReset();
    themeMocks.loadThemeCSS.mockReset();
    themeMocks.loadThemeCSS.mockResolvedValue(undefined);
    localStorage.clear();
    document.body.innerHTML = '<div id="root"></div>';
  });

  afterEach(() => {
    document.body.innerHTML = "";
    vi.useRealTimers();
  });

  // main.tsx runs createRoot(...).render(<Root/>) at module scope; re-import it
  // fresh each time so that render happens inside act.
  async function mountApp() {
    await act(async () => {
      vi.resetModules();
      await import("./main");
    });
    // Let the lazy DropdownMenu mock resolve and settle effects.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
      await vi.advanceTimersByTimeAsync(0);
      await vi.advanceTimersByTimeAsync(0);
    });
  }

  // loadThemeDef returns a controllable promise per theme id, recorded in a map
  // so the test can resolve loads out of order.
  function deferredLoaders(): Map<string, Array<(def: unknown) => void>> {
    const pending = new Map<string, Array<(def: unknown) => void>>();
    themeMocks.loadThemeDef.mockImplementation((id: string) => {
      return new Promise<unknown>((resolve) => {
        const list = pending.get(id) ?? [];
        list.push(resolve);
        pending.set(id, list);
      });
    });
    return pending;
  }

  function clickTheme(label: string) {
    const btn = Array.from(document.querySelectorAll("button")).find(
      (b) => b.getAttribute("data-theme-item") === label,
    );
    expect(btn).toBeTruthy();
    act(() => btn!.click());
  }

  it("does not revert the theme when an earlier selection resolves after a newer one", async () => {
    const pending = deferredLoaders();
    await mountApp();

    // Pick Tokyo Night first (its load will resolve last), then Flexoki.
    clickTheme("Tokyo Night");
    clickTheme("Flexoki");

    // The newer selection (Flexoki) resolves first…
    await act(async () => {
      for (const resolve of pending.get("flexoki") ?? []) resolve({ id: "flexoki" });
      await vi.advanceTimersByTimeAsync(0);
    });

    // …then the earlier one (Tokyo Night) resolves last. It must not win:
    // the theme has to stay on the latest selection, Flexoki.
    await act(async () => {
      for (const resolve of pending.get("tokyo-night") ?? []) resolve({ id: "tokyo-night" });
      await vi.advanceTimersByTimeAsync(0);
    });

    const applied = document.querySelector("[data-theme-id]");
    expect(applied?.getAttribute("data-theme-id")).toBe("flexoki");
  });

  it("still applies a single selection normally", async () => {
    const pending = deferredLoaders();
    await mountApp();

    clickTheme("Tokyo Night");

    await act(async () => {
      for (const resolve of pending.get("tokyo-night") ?? []) resolve({ id: "tokyo-night" });
      await vi.advanceTimersByTimeAsync(0);
    });

    const applied = document.querySelector("[data-theme-id]");
    expect(applied?.getAttribute("data-theme-id")).toBe("tokyo-night");
  });
});
