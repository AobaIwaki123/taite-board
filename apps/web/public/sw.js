// Ribbon Clock Service Worker (Underground Venue Offline Image Cache)
// Provides Cache-First retrieval for Twitter / X image media (pbs.twimg.com).

const CACHE_NAME = 'ribonclock-images-v1';
const TWITTER_MEDIA_PREFIX = 'https://pbs.twimg.com/media/';

self.addEventListener('install', () => {
  // Activate worker immediately without waiting for existing worker to shut down
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  // Clean up outdated image cache generations
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.map((key) => {
          if (key.startsWith('ribonclock-images-') && key !== CACHE_NAME) {
            return caches.delete(key);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  const url = event.request.url;

  // Intercept GET requests to Twitter / X media assets
  if (url.startsWith(TWITTER_MEDIA_PREFIX) && event.request.method === 'GET') {
    event.respondWith(
      caches.open(CACHE_NAME).then(async (cache) => {
        // 1. Cache-first strategy: return locally stored image immediately
        const cachedResponse = await cache.match(event.request);
        if (cachedResponse) {
          return cachedResponse;
        }

        // 2. Network fallback: fetch from origin and cache response for offline resilience
        try {
          const networkResponse = await fetch(event.request);
          // Standard 200 OK or cross-origin opaque response (type: 'opaque' from no-cors img requests)
          if (networkResponse && (networkResponse.status === 200 || networkResponse.type === 'opaque')) {
            cache.put(event.request, networkResponse.clone());
          }
          return networkResponse;
        } catch (error) {
          // If offline and not in cache, let the fetch reject naturally
          throw error;
        }
      })
    );
  }
});
