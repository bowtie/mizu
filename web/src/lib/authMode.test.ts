import { describe, it, expect } from "vitest";
import { isAuthManaged } from "./authMode";

describe("isAuthManaged", () => {
  it("is true only when Mizu does not do the sign-in itself", () => {
    expect(isAuthManaged({ mode: "none" })).toBe(true);
    expect(isAuthManaged({ mode: "proxy" })).toBe(true);
    expect(isAuthManaged({ mode: "builtin" })).toBe(false);
    expect(isAuthManaged({})).toBe(false);
    expect(isAuthManaged(undefined)).toBe(false);
  });
});
