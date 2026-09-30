// Bump when the caching strategy changes; hashed build assets are versioned by their file names.
const CACHE_NAME = 'teslacost-v2';
const OFFLINE_SHELL = ['/index.html', '/manifest.json', '/favicon.svg', '/pwa-icon.svg'];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(OFFLINE_SHELL)));
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      const staleCaches = keys.filter((key) => key !== CACHE_NAME);
      await Promise.all(staleCaches.map((key) => caches.delete(key)));
      await self.clients.claim();
      // Pages rendered by a previous worker may have received stale or corrupted assets: reload them once.
      if (staleCaches.length > 0) {
        const windows = await self.clients.matchAll({ type: 'window' });
        windows.forEach((client) => client.navigate(client.url));
      }
    })()
  );
});

function isExpectedContentType(request, response) {
  const path = new URL(typeof request === 'string' ? request : request.url, self.location.origin).pathname;
  const type = response.headers.get('content-type') || '';
  if (path.endsWith('.css')) return type.includes('text/css');
  if (path.endsWith('.js')) return type.includes('javascript');
  if (path === '/index.html') return type.includes('text/html');
  return true;
}

function putInCache(request, response) {
  // Never store an HTML fallback under a CSS/JS URL
  if (response && response.status === 200 && response.type === 'basic' && isExpectedContentType(request, response)) {
    const copy = response.clone();
    caches.open(CACHE_NAME)
      .then((cache) => cache.put(request, copy))
      .catch(() => {}); // A failed cache write (quota) only loses the offline copy
  }
  return response;
}

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const url = new URL(event.request.url);

  // Never cache API calls
  if (url.origin !== self.location.origin || url.pathname.startsWith('/api')) {
    return;
  }

  // Navigations: network first so a new deployment is picked up immediately, cached shell when offline
  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request)
        .then((response) => putInCache('/index.html', response))
        .catch(() => caches.match('/index.html'))
    );
    return;
  }

  // Hashed build assets are immutable: cache first
  if (url.pathname.startsWith('/assets/')) {
    event.respondWith(
      caches.match(event.request).then((cached) => cached || fetch(event.request).then((response) => putInCache(event.request, response)))
    );
    return;
  }

  // Other static files: network first, cache fallback
  event.respondWith(
    fetch(event.request)
      .then((response) => putInCache(event.request, response))
      .catch(() => caches.match(event.request))
  );
});
