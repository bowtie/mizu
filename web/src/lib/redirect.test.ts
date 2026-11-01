import { describe, it, expect } from "vitest";
import { safeRedirectPath } from "./redirect";

describe("safeRedirectPath", () => {
  it("keeps in-app paths, including query strings", () => {
    expect(safeRedirectPath("/settings")).toBe("/settings");
    expect(safeRedirectPath("/shows/3?tab=episodes")).toBe("/shows/3?tab=episodes");
  });

  it("falls back to the library for missing or off-site targets", () => {
    for (const bad of [
      null,
      "",
      "settings",
      "https://evil.example",
      "//evil.example",
      "/\\evil.example",
    ]) {
      expect(safeRedirectPath(bad)).toBe("/");
    }
  });
});
