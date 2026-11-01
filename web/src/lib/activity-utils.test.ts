import { describe, it, expect } from "vitest";
import {
  formatDuration,
  formatTrigger,
  humanSummary,
  detailLine,
  hasDetails,
} from "./activity-utils";
import type { SyncLog, SyncLogItem } from "./types";

describe("formatDuration", () => {
  it("returns 'Running' when startedAt is null", () => {
    expect(formatDuration(null, "2026-01-01T00:00:01Z")).toBe("Running");
  });

  it("returns 'Running' when completedAt is null", () => {
    expect(formatDuration("2026-01-01T00:00:00Z", null)).toBe("Running");
  });

  it("returns 'Running' when both are undefined", () => {
    expect(formatDuration(undefined, undefined)).toBe("Running");
  });

  it("formats sub-second durations in ms", () => {
    expect(formatDuration("2026-01-01T00:00:00.000Z", "2026-01-01T00:00:00.500Z")).toBe("500ms");
  });

  it("formats sub-minute durations in seconds", () => {
    expect(formatDuration("2026-01-01T00:00:00Z", "2026-01-01T00:00:05Z")).toBe("5.0s");
  });

  it("formats minute+ durations", () => {
    expect(formatDuration("2026-01-01T00:00:00Z", "2026-01-01T00:01:30Z")).toBe("1m 30s");
  });
});

describe("formatTrigger", () => {
  it("returns '-' for undefined", () => {
    expect(formatTrigger(undefined)).toBe("-");
  });

  it("returns 'Manual' for manual", () => {
    expect(formatTrigger("manual")).toBe("Manual");
  });

  it("returns 'Scheduled' for scheduled", () => {
    expect(formatTrigger("scheduled")).toBe("Scheduled");
  });

  it("returns 'AFL cache refresh' for afl_cache_refresh", () => {
    expect(formatTrigger("afl_cache_refresh")).toBe("AFL cache refresh");
  });

  it("returns the raw trigger for unknown values", () => {
    expect(formatTrigger("custom_trigger")).toBe("custom_trigger");
  });
});

describe("humanSummary", () => {
  const baseLog: SyncLog = {
    id: 1,
    triggered_at: "2026-01-01T00:00:00Z",
    trigger: "manual",
    shows_processed: 5,
    episodes_updated: 3,
  } as SyncLog;

  it("summarizes afl_cache_refresh with new shows", () => {
    expect(humanSummary({ ...baseLog, trigger: "afl_cache_refresh", shows_processed: 7 })).toBe(
      "7 new anime found in AFL cache",
    );
  });

  it("summarizes afl_cache_refresh with no new shows", () => {
    expect(humanSummary({ ...baseLog, trigger: "afl_cache_refresh", shows_processed: 0 })).toBe(
      "AFL cache refreshed, no new anime",
    );
  });

  it("summarizes normal sync with episode changes", () => {
    expect(humanSummary({ ...baseLog, shows_processed: 5, episodes_updated: 3 })).toBe(
      "5 anime, 3 episodes changed",
    );
  });

  it("uses singular 'episode' for 1 change", () => {
    expect(humanSummary({ ...baseLog, shows_processed: 2, episodes_updated: 1 })).toBe(
      "2 anime, 1 episode changed",
    );
  });

  it("summarizes normal sync with no episode changes", () => {
    expect(humanSummary({ ...baseLog, shows_processed: 5, episodes_updated: 0 })).toBe(
      "5 anime checked",
    );
  });
});

describe("detailLine", () => {
  it("returns the summary if present", () => {
    const item = { summary: "Custom summary" } as SyncLogItem;
    expect(detailLine(item)).toBe("Custom summary");
  });

  it("returns singular for 1 episode change", () => {
    const item = { summary: undefined, episodes_changed: 1 } as unknown as SyncLogItem;
    expect(detailLine(item)).toBe("1 episode change.");
  });

  it("returns plural for multiple episode changes", () => {
    const item = { summary: undefined, episodes_changed: 5 } as unknown as SyncLogItem;
    expect(detailLine(item)).toBe("5 episode changes.");
  });

  it("returns 'No episode changes.' for 0 changes", () => {
    const item = { summary: undefined, episodes_changed: 0 } as unknown as SyncLogItem;
    expect(detailLine(item)).toBe("No episode changes.");
  });
});

describe("hasDetails", () => {
  it("returns true when items exist", () => {
    expect(hasDetails({ id: 1, items: [{ id: 1 }] } as SyncLog)).toBe(true);
  });

  it("returns true when errors exist", () => {
    expect(hasDetails({ id: 1, items: [], errors: "something failed" } as SyncLog)).toBe(true);
  });

  it("returns false when no items and no errors", () => {
    expect(hasDetails({ id: 1, items: [], errors: undefined } as SyncLog)).toBe(false);
  });

  it("returns false when items is undefined", () => {
    expect(hasDetails({ id: 1 } as SyncLog)).toBe(false);
  });
});
