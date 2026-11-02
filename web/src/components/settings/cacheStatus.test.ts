import { describe, it, expect } from "vitest";
import { cacheStatus } from "./cacheStatus";

const now = new Date("2026-10-29T12:00:00Z");

describe("cacheStatus", () => {
  it("shows the relative refresh time for a real timestamp", () => {
    expect(cacheStatus(42, "2026-10-29T09:00:00Z", now)).toBe(
      "42 shows cached, refreshed 3 hours ago.",
    );
  });

  it.each([null, undefined, ""])("treats %j as never refreshed", (value) => {
    expect(cacheStatus(0, value, now)).toBe("0 shows cached. Never refreshed.");
  });

  it("treats Go's zero time as never refreshed", () => {
    expect(cacheStatus(0, "0001-01-01T00:00:00Z", now)).toBe("0 shows cached. Never refreshed.");
  });

  it("treats an unparseable date as never refreshed", () => {
    expect(cacheStatus(3, "not-a-date", now)).toBe("3 shows cached. Never refreshed.");
  });
});
