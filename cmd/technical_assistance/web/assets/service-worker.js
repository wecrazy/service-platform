// // service-worker.js
// const CACHE_NAME = 'v1';
// const urlsToCache = [
//     // Initial URLs to cache if any
// ];

// // Install event: cache initial assets
// self.addEventListener('install', function(event) {
//     event.waitUntil(
//         caches.open(CACHE_NAME).then(function(cache) {
//             return cache.addAll(urlsToCache);
//         })
//     );
//     self.skipWaiting();
// });

// // Activate event: clean up old caches if necessary
// self.addEventListener('activate', function(event) {
//     event.waitUntil(
//         caches.keys().then(function(cacheNames) {
//             return Promise.all(
//                 cacheNames.map(function(cacheName) {
//                     if (cacheName !== CACHE_NAME) {
//                         return caches.delete(cacheName);
//                     }
//                 })
//             );
//         })
//     );
//     self.clients.claim();
// });

// // Fetch event: serve cached assets or fetch from network
// self.addEventListener('fetch', function(event) {
//     event.respondWith(
//         caches.match(event.request).then(function(response) {
//             return response || fetch(event.request);
//         })
//     );
// });

// // Message event: cache additional URLs sent from the main script
// self.addEventListener('message', function(event) {
//     if (event.data.action === 'CACHE_URLS') {
//         caches.open(CACHE_NAME).then(function(cache) {
//             return cache.addAll(event.data.urls);
//         });
//     }
// });
