import { StrictMode, useState, useEffect, useCallback, useRef, lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { initSounds } from "./lib/sounds";
import { prefetchActivityChunk, prefetchSettingsChunk } from "./lib/chunkPrefetch";
import { logout } from "./lib/api";
import { queryClient } from "./lib/queryClient";
import { useAuthStatus } from "./lib/queries";
import { Theme } from "@astryxdesign/core/theme";
import { AppShell } from "@astryxdesign/core/AppShell";
import { TopNav } from "@astryxdesign/core/TopNav";
import { TopNavHeading } from "@astryxdesign/core/TopNav";
import { TopNavItem } from "@astryxdesign/core/TopNav";
import { NavIcon } from "@astryxdesign/core/NavIcon";
import { Button } from "@astryxdesign/core/Button";
import { HStack } from "@astryxdesign/core/HStack";
import { Check, Palette, LogOut } from "lucide-react";
import { BrowserRouter, useLocation, useNavigate, Link } from "react-router-dom";

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
import "./lib/flexokiTheme.css";

import "./fonts.css";
import "./index.css";
import { App } from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { themeMetas, defaultTheme, loadThemeCSS, loadThemeDef } from "./lib/themes";

// Key the ErrorBoundary by the current path so navigating to a different
// route automatically resets a crashed boundary — no manual reload needed.
function RouteBoundary({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation();
  return <ErrorBoundary key={pathname}>{children}</ErrorBoundary>;
}
import type { DefinedTheme } from "@astryxdesign/core/theme";

// Fire ShowsPage import immediately — it's the landing page and will
// be needed on first render. Starting the download in parallel with
// the main bundle parse avoids a waterfall.
void import("./routes/ShowsPage");

// On a hard refresh of a show page, start downloading its chunk right away
// instead of waiting for the router to request it.
if (/^\/shows\/\d+/.test(location.pathname)) {
  void import("./routes/ShowDetailPage");
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
          title="Theme"
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
          label: `Theme: ${selectedMeta.label}`,
          variant: "secondary",
          size: "sm",
          isIconOnly: true,
          icon: <Palette size={16} />,
          tooltip: `Theme: ${selectedMeta.label}`,
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

function LogoutButton() {
  const { data: status } = useAuthStatus();
  const navigate = useNavigate();

  // Nothing to sign out of while no password is set.
  if (!status?.authenticated) return null;

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // Session may already be gone — clearing local state is enough.
    } finally {
      queryClient.clear();
      navigate("/login", { replace: true });
    }
  };

  return (
    <Button
      variant="secondary"
      size="sm"
      isIconOnly
      label="Sign out"
      icon={<LogOut size={16} />}
      aria-label="Sign out"
      tooltip="Sign out"
      onClick={() => void handleLogout()}
    />
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
          <TopNavItem
            label="Activity"
            href="/activity"
            isSelected={isActivity}
            as={Link}
            onMouseEnter={prefetchActivityChunk}
          />
          <TopNavItem
            label="Settings"
            href="/settings"
            isSelected={isSettings}
            as={Link}
            onMouseEnter={prefetchSettingsChunk}
          />
        </>
      }
      endContent={
        <HStack gap={2} align="center">
          {themeMenu}
          <LogoutButton />
        </HStack>
      }
    />
  );
}

// Setup and sign-in are focused, standalone screens: no app navigation, and
// no top-nav slot at all (an empty slot still reserves a bar).
function Shell({ themeMenu }: { themeMenu: React.ReactNode }) {
  const { pathname } = useLocation();
  const standalone = pathname === "/welcome" || pathname === "/login";
  return (
    <AppShell
      variant="surface"
      height="auto"
      topNav={standalone ? undefined : <Nav themeMenu={themeMenu} />}
    >
      <RouteBoundary>
        <App />
      </RouteBoundary>
    </AppShell>
  );
}

function Root() {
  // Store only the theme ID — the actual theme definition is loaded
  // lazily. This keeps custom theme tokens out of the initial bundle
  // for users who stick with the default (flexoki) theme.
  const [themeId, setThemeId] = useState<string>(() => {
    return localStorage.getItem(STORAGE_KEY) ?? defaultTheme.id;
  });
  const [theme, setTheme] = useState<DefinedTheme>(defaultTheme.theme);
  const [themeMode, setThemeMode] = useState<"dark" | "light" | "system">("system");

  // Generation counter for theme loads. Every selection bumps it; a resolution
  // only applies if it's still the latest selection — otherwise a slower
  // earlier pick could resolve after a newer one and revert the theme.
  const themeLoadGen = useRef(0);

  // On mount, load the saved theme's definition + CSS if it's a custom
  // theme. The eager theme (flexoki) is available immediately.
  useEffect(() => {
    const meta = themeMetas.find((t) => t.id === themeId);
    if (meta?.mode) setThemeMode(meta.mode);

    let cancelled = false;
    const gen = ++themeLoadGen.current;
    Promise.all([loadThemeDef(themeId), loadThemeCSS(themeId)]).then(([def]) => {
      if (cancelled) return;
      if (themeLoadGen.current !== gen) return; // superseded by a newer selection
      setTheme(def);
    });
    return () => {
      cancelled = true;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSelect = useCallback((id: string) => {
    const meta = themeMetas.find((t) => t.id === id);
    setThemeId(id);
    localStorage.setItem(STORAGE_KEY, id);
    if (meta?.mode) setThemeMode(meta.mode);
    const gen = ++themeLoadGen.current;
    Promise.all([loadThemeDef(id), loadThemeCSS(id)]).then(([def]) => {
      if (themeLoadGen.current !== gen) return; // superseded by a newer selection
      setTheme(def);
    });
  }, []);

  // Wire cuelume interaction sounds once on mount — deferred to idle
  // time so it doesn't compete with first paint.
  useEffect(() => {
    const ric =
      "requestIdleCallback" in window
        ? (cb: () => void) => (window as Window).requestIdleCallback(cb, { timeout: 2000 })
        : (cb: () => void) => setTimeout(cb, 100);
    ric(() => initSounds());
  }, []);

  const menu = <ThemeMenu selectedId={themeId} onSelect={handleSelect} />;

  return (
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <Theme theme={theme} mode={themeMode}>
          <BrowserRouter>
            <Shell themeMenu={menu} />
          </BrowserRouter>
        </Theme>
        {import.meta.env.DEV && (
          <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-left" />
        )}
      </QueryClientProvider>
    </StrictMode>
  );
}

createRoot(document.getElementById("root")!).render(<Root />);
