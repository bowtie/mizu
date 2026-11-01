import { StrictMode, useState, useEffect } from "react";
import { createRoot } from "react-dom/client";
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

import "./index.css";
import { App } from "./App";
import { themeOptions, defaultTheme } from "./lib/themes";
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
          logo={<NavIcon icon={<span style={{ fontSize: 18, fontWeight: 700, lineHeight: 1 }}>~</span>} />}
        />
      }
      centerContent={
        <>
          <TopNavItem label="Library" href="/" isSelected={isShows} as={Link} />
          <TopNavItem label="Activity" href="/activity" isSelected={isActivity} as={Link} />
          <TopNavItem label="Settings" href="/settings" isSelected={isSettings} as={Link} />
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

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, selected.id);
  }, [selected.id]);

  const menu = <ThemeMenu selected={selected} onSelect={setSelected} />;

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
