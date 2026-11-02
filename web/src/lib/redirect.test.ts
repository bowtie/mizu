import { describe, it, expect } from "vitest";
import { rememberReturnPath, safeRedirectPath, takeReturnPath } from "./redirect";

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

describe("return path", () => {
  it("returns the remembered path once, then the library", () => {
    rememberReturnPath("/shows/3");
    expect(takeReturnPath()).toBe("/shows/3");
    expect(takeReturnPath()).toBe("/");
  });

  it("never returns an off-site target", () => {
    rememberReturnPath("//evil.example");
    expect(takeReturnPath()).toBe("/");
  });
});
