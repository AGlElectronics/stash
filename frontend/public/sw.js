// Stash service worker. Caches the static bundle so the app loads on slow
// venue Wi-Fi. API responses are NOT cached: Stash needs a connection to
// work, and an offline mode is not planned.
//
// Strategy:
//   /api/...          -> not handled, browser does default fetch
//   /static/...       -> cache-first (rsbuild emits hashed filenames here,
//                        so a fresh deploy produces fresh URLs and old
//                        cached bytes simply stop being requested)
//   everything else   -> network-first with cache fallback. This is the
//                        SPA shell (/, /index.html, /manifest.webmanifest,
//                        /sw.js itself, icons). Network-first is critical:
//                        the previous cache-first version pinned an old
//                        index.html on phones forever, which then pointed
//                        at /static/<old-hash>.js URLs that 404 after a
//                        redeploy and the app would not load until the SW
//                        was manually unregistered.
//
// Bump CACHE_NAME on any deploy where you want every client to discard the
// previous SW's caches. Old caches are purged on activate.

const CACHE_NAME = 'stash-static-v3';
const PRECACHE_PATHS = ['/', '/manifest.webmanifest'];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE_PATHS)).catch(() => undefined)
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

// Allow the page to force an immediate update after a deploy:
//   navigator.serviceWorker.controller?.postMessage({ type: 'SKIP_WAITING' })
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith('/api/')) return;

  // Hashed assets emitted by rsbuild are immutable per build, so cache-first
  // is safe and avoids re-downloading the same bytes on every navigation.
  if (url.pathname.startsWith('/static/')) {
    event.respondWith(cacheFirst(req));
    return;
  }

  // Everything else (SPA shell + manifest + icons): network-first. If the
  // network is reachable we always serve fresh bytes, so a deploy is
  // immediately visible. Falls back to cache when offline.
  event.respondWith(networkFirst(req));
});

async function cacheFirst(req) {
  const cached = await caches.match(req);
  if (cached) return cached;
  try {
    const res = await fetch(req);
    if (res && res.status === 200 && res.type === 'basic') {
      const copy = res.clone();
      caches.open(CACHE_NAME).then((cache) => cache.put(req, copy)).catch(() => undefined);
    }
    return res;
  } catch (e) {
    return cached ?? Response.error();
  }
}

async function networkFirst(req) {
  try {
    const res = await fetch(req);
    if (res && res.status === 200 && res.type === 'basic') {
      const copy = res.clone();
      caches.open(CACHE_NAME).then((cache) => cache.put(req, copy)).catch(() => undefined);
    }
    return res;
  } catch (e) {
    const cached = await caches.match(req);
    if (cached) return cached;
    // For navigations, fall back to the cached SPA shell so a phone with no
    // signal still opens the last-known UI instead of the browser error page.
    if (req.mode === 'navigate') {
      const shell = await caches.match('/');
      if (shell) return shell;
    }
    return Response.error();
  }
}
