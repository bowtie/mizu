import { describe, it, expect } from "vitest";
import { errorMessage, getArtworkBannerUrl, getArtworkLogoUrl, getArtworkPosterUrl } from "./api";

describe("errorMessage", () => {
  it("returns the message for a plain Error", () => {
    expect(errorMessage(new Error("something broke"))).toBe("something broke");
  });

  it("strips Go-style Post prefix", () => {
    const msg = errorMessage(
      new Error('Post "http://localhost:8978/api/settings": dial tcp: connection refused'),
    );
    expect(msg).toBe("connection was refused by the server");
  });

  it("strips Go-style Get prefix", () => {
    const msg = errorMessage(
      new Error('Get "http://localhost:8978/api/mappings": dial tcp: connection refused'),
    );
    expect(msg).toBe("connection was refused by the server");
  });

  it("converts 'no such host' to friendlier message", () => {
    const msg = errorMessage(
      new Error(
        'Get "http://sonarr.example:8989/api/series": dial tcp: lookup sonarr.example: no such host',
      ),
    );
    expect(msg).toBe("sonarr.example: could not be reached");
  });

  it("converts 'i/o timeout' to friendlier message", () => {
    const msg = errorMessage(
      new Error('Get "http://slow.example:8989/api": dial tcp [1.2.3.4]:8989: i/o timeout'),
    );
    expect(msg).toBe("connection timed out");
  });

  it("converts 'context deadline exceeded' to friendlier message", () => {
    const msg = errorMessage(
      new Error("context deadline exceeded (client timeout exceeded while awaiting headers)"),
    );
    expect(msg).toBe("request timed out");
  });

  it("returns string representation for non-Error values", () => {
    expect(errorMessage("plain string")).toBe("plain string");
    expect(errorMessage(42)).toBe("42");
    expect(errorMessage(null)).toBe("null");
  });

  it("preserves messages without Go-style prefixes", () => {
    expect(errorMessage(new Error("not found"))).toBe("not found");
  });
});

describe("artwork URLs", () => {
  it("are plain without a version", () => {
    expect(getArtworkPosterUrl(3)).toBe("/api/artwork/3/poster");
  });

  it("carry a version that is stable for the same image and changes with it", () => {
    const a = getArtworkBannerUrl(3, "https://assets.fanart.tv/a.jpg");
    expect(a).toMatch(/^\/api\/artwork\/3\/banner\?v=[0-9a-z]+$/);
    expect(getArtworkBannerUrl(3, "https://assets.fanart.tv/a.jpg")).toBe(a);
    expect(getArtworkBannerUrl(3, "https://assets.fanart.tv/b.jpg")).not.toBe(a);
    expect(getArtworkLogoUrl(3, "x")).toMatch(/^\/api\/artwork\/3\/logo\?v=/);
  });
});
