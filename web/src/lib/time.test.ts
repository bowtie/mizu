import { describe, it, expect } from "vitest";
import { formatRelativeTime, isSyncOverdue } from "./time";

const now = new Date("2026-10-29T12:00:00Z");

describe("formatRelativeTime", () => {
  it("collapses very recent times", () => {
    expect(formatRelativeTime("2026-10-29T11:59:30Z", now)).toBe("just now");
  });
  it("uses the largest fitting unit", () => {
    expect(formatRelativeTime("2026-10-29T09:00:00Z", now)).toBe("3 hours ago");
    expect(formatRelativeTime("2026-10-26T12:00:00Z", now)).toBe("3 days ago");
  });
});

describe("isSyncOverdue", () => {
  it("flags syncs older than two intervals", () => {
    expect(isSyncOverdue("2026-10-29T01:00:00Z", "6h", now)).toBe(false);
    expect(isSyncOverdue("2026-10-28T20:00:00Z", "6h", now)).toBe(true);
  });
  it("falls back to 6h for unknown intervals", () => {
    expect(isSyncOverdue("2026-10-28T20:00:00Z", "weird", now)).toBe(true);
  });
});
