// service-worker.js — Offline PWA cache for go-comic-converter WASM app
// Cache-first for .wasm binary, stale-while-revalidate for static assets.

const CACHE_NAME = 'go-comic-converter-v1';

// Assets to pre-cache on install
const PRECACHE_ASSETS = [
  '/',
  'index.html',
  'app.js',
  'wasm_exec.js',
  'memfs.js',
  'worker.js',
  'manifest.json',
  'icons/icon-192.png',
  'icons/icon-512.png',
];

self.addEventListener('install', function (event) {
  event.waitUntil(
    caches.open(CACHE_NAME).then(function (cache) {
      return cache.addAll(PRECACHE_ASSETS);
    }).then(function () {
      return self.skipWaiting();
    })
  );
});

self.addEventListener('activate', function (event) {
  event.waitUntil(
    caches.keys().then(function (keys) {
      return Promise.all(
        keys.filter(function (k) { return k !== CACHE_NAME; })
          .map(function (k) { return caches.delete(k); })
      );
    }).then(function () {
      return self.clients.claim();
    })
  );
});

self.addEventListener('fetch', function (event) {
  var url = new URL(event.request.url);

  // Only handle same-origin requests
  if (url.origin !== self.location.origin) return;

  // Fetch version.json to resolve content-hash WASM URL
  if (url.pathname.endsWith('.wasm')) {
    // Cache-first for WASM binaries
    event.respondWith(
      caches.match(event.request).then(function (cached) {
        if (cached) return cached;
        return fetchAndCache(event.request);
      })
    );
    return;
  }

  // For version.json and other dynamic assets, network-first
  if (url.pathname.endsWith('version.json')) {
    event.respondWith(
      fetch(event.request).catch(function () {
        return caches.match(event.request);
      })
    );
    return;
  }

  // Stale-while-revalidate for HTML, JS, CSS
  event.respondWith(
    caches.match(event.request).then(function (cached) {
      var fetchPromise = fetch(event.request).then(function (response) {
        if (response && response.status === 200) {
          var copy = response.clone();
          caches.open(CACHE_NAME).then(function (cache) {
            cache.put(event.request, copy);
          });
        }
        return response;
      }).catch(function () {
        return cached;
      });
      return cached || fetchPromise;
    })
  );
});

function fetchAndCache(request) {
  return fetch(request).then(function (response) {
    if (response && response.status === 200) {
      var copy = response.clone();
      caches.open(CACHE_NAME).then(function (cache) {
        cache.put(request, copy);
      });
    }
    return response;
  });
}
