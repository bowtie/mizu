// Mizu Service Worker — v2
// Strategies:
//   /assets/, /fonts/  → cache-first (immutable hashed files)
//   /api/              → stale-while-revalidate (except auth + sensitive)
//   navigation         → network-first, fallback to cached index.html

const CACHE_VERSION = "mizu-v2";
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
          const copy = res.clone();
          caches.open(ASSET_CACHE).then((c) => c.put("/index.html", copy));
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

  // API responses — stale-while-revalidate.
  // Skip caching for endpoints that return sensitive data (API keys).
  if (url.pathname.startsWith("/api/")) {
    // Never cache settings or bootstrap — they contain full API keys.
    // Never handle /api/auth/* at all (no cache read, no write, no
    // offline fallback): a stale auth status makes the login page see
    // {authenticated: true} after logout or a password change and
    // bounce straight back — a redirect loop. Login state must be live.
    const isSensitive = url.pathname === "/api/settings" || url.pathname === "/api/bootstrap";
    const isAuth = url.pathname.startsWith("/api/auth/");
    if (isSensitive) {
      event.respondWith(fetch(req).catch(() => caches.match(req)));
      return;
    }
    if (isAuth) {
      event.respondWith(fetch(req));
      return;
    }
    event.respondWith(
      caches.match(req).then((cached) => {
        const networkFetch = fetch(req)
          .then((res) => {
            if (res.ok) {
              const copy = res.clone();
              caches.open(API_CACHE).then((c) => c.put(req, copy));
            }
            return res;
          })
          .catch(() => cached);
        return cached || networkFetch;
      }),
    );
    return;
  }
});
