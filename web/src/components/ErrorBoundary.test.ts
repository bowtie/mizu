import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MemoryRouter, useLocation, useNavigate } from "react-router-dom";
import { ErrorBoundary } from "./ErrorBoundary";

// React logs errors caught by an error boundary to console.error; silence
// that so test output stays clean and assert on behavior instead.
beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});
afterEach(() => {
  vi.restoreAllMocks();
});

// A child that throws during render.
function Bomb(): ReactNode {
  throw new Error("kaboom");
}

// A child that can be made to throw from outside React, so tests can
// "fix" it and then exercise Try again / route-change recovery.
let crash = false;
function Crashable() {
  if (crash) throw new Error("kaboom");
  return createElement("div", { "data-testid": "healthy" }, "healthy content");
}

// Mirrors the RouteBoundary wrapper in main.tsx: the boundary is keyed by
// pathname so navigating to a different route remounts it and clears the
// error state.
function RouteBoundary({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  return createElement(ErrorBoundary, { key: pathname, children });
}

function RouteApp() {
  const navigate = useNavigate();
  return createElement(
    "div",
    null,
    createElement("button", { onClick: () => navigate("/other") }, "go to other route"),
    createElement(RouteBoundary, null, createElement(Crashable)),
  );
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  root.unmount();
  container.remove();
  crash = false;
});

describe("ErrorBoundary", () => {
  it("renders the fallback UI when a child throws during render", async () => {
    await act(async () => {
      root.render(createElement(ErrorBoundary, null, createElement(Bomb)));
    });

    expect(container.querySelector('[role="alert"]')).not.toBeNull();
    expect(container.textContent).toContain("This page ran into a problem");
    expect(container.textContent).toContain("Reload page");
    // The error message is available under "Technical details".
    expect(container.textContent).toContain("Technical details");
    expect(container.innerHTML).toContain("kaboom");
  });

  it("renders children normally when no error occurs", async () => {
    await act(async () => {
      root.render(createElement(ErrorBoundary, null, createElement("div", null, "all good")));
    });

    expect(container.textContent).toContain("all good");
    expect(container.querySelector('[role="alert"]')).toBeNull();
  });

  it("offers Go to library off the library, and it recovers the page", async () => {
    crash = true;
    await act(async () => {
      root.render(
        createElement(MemoryRouter, { initialEntries: ["/settings"] }, createElement(RouteApp)),
      );
    });
    expect(container.querySelector('[role="alert"]')).not.toBeNull();

    crash = false;
    const goHome = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Go to library"),
    );
    expect(goHome).toBeDefined();
    await act(async () => {
      goHome!.click();
    });

    expect(container.querySelector('[role="alert"]')).toBeNull();
    expect(container.textContent).toContain("healthy content");
  });

  it("hides Go to library when the library itself crashed", async () => {
    crash = true;
    await act(async () => {
      root.render(createElement(MemoryRouter, { initialEntries: ["/"] }, createElement(RouteApp)));
    });
    expect(container.textContent).toContain("Reload page");
    expect(container.textContent).not.toContain("Go to library");
  });

  it("recovers on route change (key remount clears error state)", async () => {
    crash = true;
    await act(async () => {
      root.render(createElement(MemoryRouter, null, createElement(RouteApp)));
    });
    expect(container.textContent).toContain("This page ran into a problem");

    // Fix the child and navigate to a different route — the boundary
    // remounts under a new pathname key with fresh state.
    crash = false;
    const navButton = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("go to other route"),
    );
    expect(navButton).toBeDefined();
    await act(async () => {
      navButton!.click();
    });

    expect(container.querySelector('[role="alert"]')).toBeNull();
    expect(container.textContent).toContain("healthy content");
  });
});
