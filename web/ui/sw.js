// OffGrid LLM Service Worker
// Enables offline capability and caching

const CACHE_NAME = 'offgrid-v0.2.12-security-1';
const STATIC_ASSETS = [
    '/ui/',
    '/ui/index.html',
    '/ui/manifest.json',
    '/ui/css/styles.css',
    '/ui/vendor/css/tailwind.min.css',
    '/ui/vendor/css/atom-one-dark.min.css',
    '/ui/vendor/js/highlight.min.js',
    '/ui/vendor/js/highlight-python.min.js',
    '/ui/vendor/js/highlight-javascript.min.js',
    '/ui/vendor/js/highlight-java.min.js',
    '/ui/vendor/js/highlight-cpp.min.js',
    '/ui/vendor/js/highlight-bash.min.js',
    '/ui/vendor/js/marked.min.js',
    '/ui/vendor/js/purify.min.js',
    '/ui/js/themes.js',
    '/ui/js/utils.js',
    '/ui/js/model-manager.js',
    '/ui/js/onboarding.js',
    '/ui/js/status.js',
    '/ui/js/modals.js',
    '/ui/js/models.js',
    '/ui/js/chat.js',
    '/ui/js/chat-ui.js',
    '/ui/js/models-ui.js',
    '/ui/js/terminal.js',
    '/ui/js/rag.js',
    '/ui/js/benchmark.js',
    '/ui/js/auth.js',
    '/ui/js/agent.js',
    '/ui/js/users.js',
    '/ui/js/metrics.js',
    '/ui/js/lora.js',
    '/ui/js/audio.js',
    '/ui/js/file-browser.js',
    '/ui/js/p2p.js',
    '/ui/js/mobile-nav.js'
];

// Install event - cache static assets
self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then(cache => cache.addAll(STATIC_ASSETS))
            .then(() => self.skipWaiting())
    );
});

// Activate event - clean old caches
self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys => {
            return Promise.all(
                keys.filter(key => key !== CACHE_NAME)
                    .map(key => caches.delete(key))
            );
        }).then(() => self.clients.claim())
    );
});

// Fetch event - serve from cache, fallback to network
self.addEventListener('fetch', event => {
    const url = new URL(event.request.url);
    
    // Skip caching for API calls
    if (url.pathname.startsWith('/v1/')) {
        return event.respondWith(fetch(event.request));
    }
    
    // Cache-first for static assets
    event.respondWith(
        caches.match(event.request, { ignoreSearch: true })
            .then(cached => cached || fetch(event.request))
            .catch(() => {
                // Return cached index.html for navigation requests
                if (event.request.mode === 'navigate') {
                    return caches.match('/ui/index.html');
                }
            })
    );
});
