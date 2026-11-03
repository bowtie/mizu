import { describe, it, expect } from "vitest";
import { slugify, showSlugs, showPath, resolveShowRef } from "./slug";

const SHOWS = [
  { id: 67, sonarr_title: "Cardcaptor Sakura" },
  { id: 57, sonarr_title: "Pokémon" },
  { id: 3, sonarr_title: "Hunter x Hunter" },
  { id: 4, sonarr_title: "Hunter x Hunter" },
  { id: 9, sonarr_title: "!!!" },
];

describe("show slugs", () => {
  it("slugs titles to lowercase ASCII words", () => {
    expect(slugify("Cardcaptor Sakura")).toBe("cardcaptor-sakura");
    expect(slugify("Pokémon")).toBe("pokemon");
    expect(slugify("Kaguya-sama: Love Is War")).toBe("kaguya-sama-love-is-war");
  });

  it("keeps slugs unique and non-empty", () => {
    const slugs = showSlugs(SHOWS);
    expect(slugs.get("67")).toBe("cardcaptor-sakura");
    expect(slugs.get("3")).toBe("hunter-x-hunter-3");
    expect(slugs.get("4")).toBe("hunter-x-hunter-4");
    expect(slugs.get("9")).toBe("show-9");
  });

  it("builds paths, falling back to the id for an unknown show", () => {
    expect(showPath(67, SHOWS)).toBe("/shows/cardcaptor-sakura");
    expect(showPath(99, SHOWS)).toBe("/shows/99");
    expect(showPath(67, undefined)).toBe("/shows/67");
  });

  it("resolves slugs and old ids", () => {
    expect(resolveShowRef("pokemon", SHOWS)).toBe("57");
    expect(resolveShowRef("67", undefined)).toBe("67");
    expect(resolveShowRef("nope", SHOWS)).toBeUndefined();
    expect(resolveShowRef("pokemon", undefined)).toBeUndefined();
  });
});
