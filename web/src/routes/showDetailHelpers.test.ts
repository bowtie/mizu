import { describe, expect, it } from "vitest";
import type { Episode } from "../lib/types";
import { episodeNumberText } from "./showDetailHelpers";

describe("episodeNumberText", () => {
  it("uses the AFL-aligned number for an episode AFL counts", () => {
    const ep = {
      season_number: 1,
      sonarr_episode_number: 19,
      absolute_episode_number: 18,
    } as Episode;
    expect(episodeNumberText(ep)).toBe("Episode 18");
  });

  it("uses Sonarr's season and episode for one AFL doesn't count", () => {
    const ep = {
      season_number: 1,
      sonarr_episode_number: 18,
      absolute_episode_number: 18,
      not_on_afl: true,
    } as Episode;
    expect(episodeNumberText(ep)).toBe("S01E18");
  });
});
