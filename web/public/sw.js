// Mizu Service Worker — v3
// Strategies:
//   /assets/, /fonts/  → cache-first (immutable hashed files)
//   /api/              → network-first, cached copy only when offline
//                        (auth, backup and status polls never cached)
//   navigation         → network-first, fallback to cached index.html

const CACHE_VERSION = "mizu-v3";
const ASSET_CACHE = CACHE_VERSION + "-assets";
const API_CACHE = CACHE_VERSION + "-api";

// Assets to pre-cache on install (just the offline shell).
const PRECACHE_URLS = ["/", "/index.html"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(ASSET_CACHE)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((k) => !k.startsWith(CACHE_VERSION)).map((k) => caches.delete(k))),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const url = new URL(req.url);

  // Only handle GET requests from same origin.
  if (req.method !== "GET" || url.origin !== self.location.origin) return;

  // Navigation requests — network-first, fall back to cached index.html.
  if (req.mode === "navigate") {
    event.respondWith(
      fetch(req)
        .then((res) => {
          // Only a real page may become the offline shell — never an error
          // page from the server or a proxy in front of it.
          if (res.ok && (res.headers.get("Content-Type") || "").includes("text/html")) {
            const copy = res.clone();
            caches.open(ASSET_CACHE).then((c) => c.put("/index.html", copy));
          }
          return res;
        })
        .catch(() => caches.match("/index.html").then((r) => r || caches.match("/"))),
    );
    return;
  }

  // Immutable hashed assets — cache-first.
  if (url.pathname.startsWith("/assets/") || url.pathname.startsWith("/fonts/")) {
    event.respondWith(
      caches.match(req).then((cached) => {
        if (cached) return cached;
        return fetch(req).then((res) => {
          if (res.ok) {
            const copy = res.clone();
            caches.open(ASSET_CACHE).then((c) => c.put(req, copy));
          }
          return res;
        });
      }),
    );
    return;
  }

  // API responses — network-first. Serving a cached copy first would show
  // stale data right after a change and make status polls report an old
  // refresh; the cache is only an offline fallback.
  if (url.pathname.startsWith("/api/")) {
    // Never store: auth state must be live (a stale status loops the login
    // page), the backup export contains raw API keys, and the status
    // endpoints are polled for live progress.
    const noStore =
      url.pathname.startsWith("/api/auth/") ||
      url.pathname.startsWith("/api/backup/") ||
      url.pathname.endsWith("/status") ||
      url.pathname === "/api/health";
    if (noStore) {
      event.respondWith(fetch(req));
      return;
    }
    event.respondWith(
      fetch(req)
        .then((res) => {
          const cacheControl = res.headers.get("Cache-Control") || "";
          if (res.ok && !cacheControl.includes("no-store")) {
            const copy = res.clone();
            caches.open(API_CACHE).then((c) => c.put(req, copy));
          }
          return res;
        })
        .catch(() => caches.match(req).then((cached) => cached || Response.error())),
    );
    return;
  }
});
