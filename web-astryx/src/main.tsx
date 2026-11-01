import { StrictMode, useState, useEffect, useCallback } from "react";
import { createRoot } from "react-dom/client";
import { initSounds } from "./lib/sounds";
import { prefetchActivityChunk, prefetchSettingsChunk } from "./lib/chunkPrefetch";
import { Theme } from "@astryxdesign/core/theme";
import { AppShell } from "@astryxdesign/core/AppShell";
import { TopNav } from "@astryxdesign/core/TopNav";
import { TopNavHeading } from "@astryxdesign/core/TopNav";
import { TopNavItem } from "@astryxdesign/core/TopNav";
import { NavIcon } from "@astryxdesign/core/NavIcon";
import { DropdownMenu } from "@astryxdesign/core/DropdownMenu";
import { Check, Palette } from "lucide-react";
import { BrowserRouter, useLocation, Link } from "react-router-dom";

// Import theme CSS files
import "@astryxdesign/core/reset.css";
import "@astryxdesign/core/astryx.css";
import "@astryxdesign/theme-neutral/theme.css";
import "@astryxdesign/theme-y2k/theme.css";

import "./fonts.css";
import "./index.css";
import { App } from "./App";
import { themeOptions, defaultTheme, loadThemeCSS } from "./lib/themes";
import type { ThemeOption } from "./lib/themes";

const STORAGE_KEY = "mizu-theme";

function ThemeMenu({
  selected,
  onSelect,
}: {
  selected: ThemeOption;
  onSelect: (t: ThemeOption) => void;
}) {
  const items = themeOptions.map((opt) => ({
    label: opt.label,
    icon: selected.id === opt.id ? <Check size={14} /> : undefined,
    onClick: () => onSelect(opt),
  }));

  return (
    <DropdownMenu
      button={{
        label: selected.label,
        variant: "secondary",
        size: "sm",
        isIconOnly: true,
        icon: <Palette size={16} />,
      }}
      items={items}
      menuWidth={220}
    />
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
  const [selected, setSelected] = useState<ThemeOption>(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    return themeOptions.find((t) => t.id === saved) ?? defaultTheme;
  });

  // Load CSS for the saved theme on mount (if it's a custom theme
  // whose CSS isn't bundled eagerly).
  useEffect(() => {
    loadThemeCSS(selected.id);
  }, []);

  // Persist theme choice and load CSS when the user switches themes.
  const handleSelect = useCallback((t: ThemeOption) => {
    loadThemeCSS(t.id);
    setSelected(t);
  }, []);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, selected.id);
  }, [selected.id]);

  // Wire cuelume interaction sounds once on mount — deferred to idle
  // time so it doesn't compete with first paint.
  useEffect(() => {
    const ric = "requestIdleCallback" in window
      ? (cb: () => void) => (window as Window).requestIdleCallback(cb, { timeout: 2000 })
      : (cb: () => void) => setTimeout(cb, 100);
    ric(() => initSounds());
  }, []);

  const menu = <ThemeMenu selected={selected} onSelect={handleSelect} />;

  return (
    <StrictMode>
      <Theme theme={selected.theme} mode={selected.mode ?? "system"}>
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
