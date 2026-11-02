import { afterEach, describe, it, expect, vi } from "vitest";
import {
  errorMessage,
  exportConfig,
  getArtworkBannerUrl,
  getArtworkLogoUrl,
  getArtworkPosterUrl,
  getAuthStatus,
} from "./api";

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

describe("session hygiene", () => {
  const fakeCaches = () => {
    const deleted: string[] = [];
    const store = {
      keys: vi.fn().mockResolvedValue(["mizu-v3-api", "mizu-v3-assets"]),
      delete: vi.fn((k: string) => {
        deleted.push(k);
        return Promise.resolve(true);
      }),
    };
    vi.stubGlobal("caches", store);
    return deleted;
  };

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("drops the offline API cache when a request comes back 401", async () => {
    const deleted = fakeCaches();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: "authentication required" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
    await expect(getAuthStatus()).rejects.toThrow();
    await vi.waitFor(() => expect(deleted).toEqual(["mizu-v3-api"]));
  });

  it("asks for secrets in the export only when told to", async () => {
    const seen: { method: string; body: unknown }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (req: Request) => {
        seen.push({ method: req.method, body: await req.json() });
        return new Response("{}", { status: 200 });
      }),
    );

    await exportConfig();
    await exportConfig(true, "pw");
    expect(seen).toEqual([
      { method: "POST", body: { include_secrets: false, password: "" } },
      { method: "POST", body: { include_secrets: true, password: "pw" } },
    ]);
  });
});
