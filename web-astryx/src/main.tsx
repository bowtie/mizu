import { StrictMode, useState, useEffect, useCallback, lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { initSounds } from "./lib/sounds";
import { prefetchActivityChunk, prefetchSettingsChunk } from "./lib/chunkPrefetch";
import { prefetchEpisodes, prefetchShowDetail } from "./lib/api";
import { Theme } from "@astryxdesign/core/theme";
import { AppShell } from "@astryxdesign/core/AppShell";
import { TopNav } from "@astryxdesign/core/TopNav";
import { TopNavHeading } from "@astryxdesign/core/TopNav";
import { TopNavItem } from "@astryxdesign/core/TopNav";
import { NavIcon } from "@astryxdesign/core/NavIcon";
import { Check, Palette } from "lucide-react";
import { BrowserRouter, useLocation, Link } from "react-router-dom";

// Lazy-load DropdownMenu — it pulls in usePopover, useListFocus,
// useTypeahead, and layerAnimations (~53KB source). The theme menu
// is only needed when the user clicks the palette button in the
// top-right corner, so deferring it keeps these deps out of the
// main bundle and off the first-paint critical path.
const DropdownMenu = lazy(() =>
  import("@astryxdesign/core/DropdownMenu").then((m) => ({ default: m.DropdownMenu })),
);

// Import theme CSS files
import "@astryxdesign/core/reset.css";
import "@astryxdesign/core/astryx.css";
import "@astryxdesign/theme-neutral/theme.css";

import "./fonts.css";
import "./index.css";
import { App } from "./App";
import { themeMetas, defaultTheme, loadThemeCSS, loadThemeDef } from "./lib/themes";
import type { DefinedTheme } from "@astryxdesign/core/theme";

// Fire ShowsPage import immediately — it's the landing page and will
// be needed on first render. Starting the download in parallel with
// the main bundle parse avoids a waterfall.
void import("./routes/ShowsPage");

// If the user navigated directly to a show detail page (e.g. hard
// refresh on /shows/123), start downloading the ShowDetailPage chunk
// immediately instead of waiting for the idle prefetch. This avoids a
// sequential waterfall: main bundle → parse → idle → prefetch → download.
// Also prefetch the show's episodes + show data so they load instantly
// once the detail page mounts — the promises are cached and picked up
// by getEpisodes/getShow.
const detailMatch = location.pathname.match(/^\/shows\/(\d+)/);
if (detailMatch) {
  void import("./routes/ShowDetailPage");
  // Use the combined detail endpoint — fetches show + episodes in one
  // request instead of two separate round-trips.
  prefetchShowDetail(detailMatch[1]);
}

const STORAGE_KEY = "mizu-theme";

function ThemeMenu({
  selectedId,
  onSelect,
}: {
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  const items = themeMetas.map((opt) => ({
    label: opt.label,
    icon: selectedId === opt.id ? <Check size={14} /> : undefined,
    onClick: () => onSelect(opt.id),
  }));

  const selectedMeta = themeMetas.find((t) => t.id === selectedId) ?? themeMetas[0];

  return (
    <Suspense
      fallback={
        <button
          className="theme-menu-placeholder"
          aria-label="Theme"
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            width: 32,
            height: 32,
            borderRadius: "var(--radius-inner)",
            border: "1px solid var(--color-border-subtle)",
            background: "transparent",
            cursor: "pointer",
            color: "var(--color-icon-secondary)",
          }}
        >
          <Palette size={16} />
        </button>
      }
    >
      <DropdownMenu
        button={{
          label: selectedMeta.label,
          variant: "secondary",
          size: "sm",
          isIconOnly: true,
          icon: <Palette size={16} />,
        }}
        items={items}
        menuWidth={220}
      />
    </Suspense>
  );
}

function MizuLogo() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      aria-hidden="true"
      style={{ display: "block" }}
    >
      <path d="M3 8c2-2.2 4-2.2 6 0s4 2.2 6 0 4-2.2 6 0" />
      <path d="M3 16c2-2.2 4-2.2 6 0s4 2.2 6 0 4-2.2 6 0" opacity="0.55" />
    </svg>
  );
}

function Nav({ themeMenu }: { themeMenu: React.ReactNode }) {
  const location = useLocation();
  const currentPath = location.pathname;
  const isShows =
    currentPath === "/" || currentPath === "/shows" || currentPath.startsWith("/shows/");
  const isActivity = currentPath === "/activity";
  const isSettings = currentPath === "/settings";

  return (
    <TopNav
      label="Mizu navigation"
      heading={
        <TopNavHeading
          heading="Mizu"
          headingHref="/"
          as={Link}
          logo={<NavIcon icon={<MizuLogo />} />}
        />
      }
      centerContent={
        <>
          <TopNavItem label="Library" href="/" isSelected={isShows} as={Link} />
          <TopNavItem label="Activity" href="/activity" isSelected={isActivity} as={Link} onMouseEnter={prefetchActivityChunk} />
          <TopNavItem label="Settings" href="/settings" isSelected={isSettings} as={Link} onMouseEnter={prefetchSettingsChunk} />
        </>
      }
      endContent={themeMenu}
    />
  );
}

function Root() {
  // Store only the theme ID — the actual theme definition is loaded
  // lazily. This keeps ~32KB of custom theme tokens out of the initial
  // bundle for users who stick with the default (neutral) theme.
  const [themeId, setThemeId] = useState<string>(() => {
    return localStorage.getItem(STORAGE_KEY) ?? defaultTheme.id;
  });
  const [theme, setTheme] = useState<DefinedTheme>(defaultTheme.theme);
  const [themeMode, setThemeMode] = useState<"dark" | "light" | "system">("system");

  // On mount, load the saved theme's definition + CSS if it's a custom
  // theme. Eager themes (neutral, y2k) are available immediately.
  useEffect(() => {
    const meta = themeMetas.find((t) => t.id === themeId);
    if (meta?.mode) setThemeMode(meta.mode);

    let cancelled = false;
    Promise.all([loadThemeDef(themeId), loadThemeCSS(themeId)]).then(([def]) => {
      if (cancelled) return;
      setTheme(def);
    });
    return () => { cancelled = true; };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSelect = useCallback((id: string) => {
    const meta = themeMetas.find((t) => t.id === id);
    setThemeId(id);
    localStorage.setItem(STORAGE_KEY, id);
    if (meta?.mode) setThemeMode(meta.mode);
    Promise.all([loadThemeDef(id), loadThemeCSS(id)]).then(([def]) => {
      setTheme(def);
    });
  }, []);

  // Wire cuelume interaction sounds once on mount — deferred to idle
  // time so it doesn't compete with first paint.
  useEffect(() => {
    const ric = "requestIdleCallback" in window
      ? (cb: () => void) => (window as Window).requestIdleCallback(cb, { timeout: 2000 })
      : (cb: () => void) => setTimeout(cb, 100);
    ric(() => initSounds());
  }, []);

  const menu = <ThemeMenu selectedId={themeId} onSelect={handleSelect} />;

  return (
    <StrictMode>
      <Theme theme={theme} mode={themeMode}>
        <BrowserRouter>
          <AppShell
            variant="surface"
            height="auto"
            topNav={<Nav themeMenu={menu} />}
          >
            <App />
          </AppShell>
        </BrowserRouter>
      </Theme>
    </StrictMode>
  );
}

createRoot(document.getElementById("root")!).render(<Root />);
