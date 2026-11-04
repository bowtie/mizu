import { describe, expect, it } from "vitest";
import type { Episode } from "../lib/types";
import { episodeBadgeInfo, episodeNumberText } from "./showDetailHelpers";

describe("episodeNumberText", () => {
  it("gives TVDB specials no number line", () => {
    expect(episodeNumberText({ season_number: 0, sonarr_episode_number: 3 } as Episode)).toBe("");
    expect(episodeNumberText({ episode_num: 61, sonarr_episode_number: 61 } as Episode)).toBe("");
  });

  it("uses the AFL-aligned number for an episode AFL counts", () => {
    const ep = {
      season_number: 1,
      sonarr_episode_number: 19,
      absolute_episode_number: 18,
    } as Episode;
    expect(episodeNumberText(ep)).toBe("Episode 18");
  });

  it("gives no number to one AFL doesn't count", () => {
    const ep = {
      season_number: 1,
      sonarr_episode_number: 18,
      absolute_episode_number: 18,
      not_on_afl: true,
    } as Episode;
    expect(episodeNumberText(ep)).toBe("Not listed");
    expect(episodeBadgeInfo(undefined, false, true).label).toBe("—");
  });
});
