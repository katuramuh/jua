package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaPWAFiles(root string, opts Options) error {
	// PWA is frontend-only — skip for API-only architectures
	if !opts.ShouldIncludeWeb() {
		return nil
	}

	webRoot := filepath.Join(root, "apps", "web")

	files := map[string]string{
		filepath.Join(webRoot, "public", "manifest.json"):           pwaManifestJSON(opts),
		filepath.Join(webRoot, "public", "sw.js"):                   pwaServiceWorkerJS(),
		filepath.Join(webRoot, "app", "manifest.ts"):                pwaNextManifestTS(opts),
		filepath.Join(webRoot, "components", "pwa", "InstallBanner.tsx"): pwaInstallBannerTSX(),
		filepath.Join(webRoot, "hooks", "usePWA.ts"):                pwsUsePWAHook(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", opts.ProjectName)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func pwaManifestJSON(opts Options) string {
	name := strings.Title(strings.ReplaceAll(opts.ProjectName, "-", " "))
	return `{
  "name": "` + name + `",
  "short_name": "` + name + `",
  "description": "Built with Jua — Go + React for Africa",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#0a0a0f",
  "theme_color": "#6c5ce7",
  "orientation": "portrait-primary",
  "categories": ["business", "productivity"],
  "icons": [
    {
      "src": "/icons/icon-72x72.png",
      "sizes": "72x72",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-96x96.png",
      "sizes": "96x96",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-128x128.png",
      "sizes": "128x128",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-144x144.png",
      "sizes": "144x144",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-152x152.png",
      "sizes": "152x152",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-192x192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-384x384.png",
      "sizes": "384x384",
      "type": "image/png",
      "purpose": "maskable any"
    },
    {
      "src": "/icons/icon-512x512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "maskable any"
    }
  ],
  "shortcuts": [
    {
      "name": "Dashboard",
      "short_name": "Dashboard",
      "description": "Go to dashboard",
      "url": "/dashboard",
      "icons": [{ "src": "/icons/icon-96x96.png", "sizes": "96x96" }]
    }
  ],
  "screenshots": [],
  "prefer_related_applications": false
}
`
}

func pwaServiceWorkerJS() string {
	return `// Jua PWA Service Worker
// Implements a cache-first strategy for static assets and network-first for API calls.

const CACHE_NAME = 'jua-cache-v1';
const STATIC_ASSETS = [
  '/',
  '/offline',
  '/manifest.json',
];

// Install: pre-cache static assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_ASSETS);
    })
  );
  self.skipWaiting();
});

// Activate: clean up old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key !== CACHE_NAME)
          .map((key) => caches.delete(key))
      )
    )
  );
  self.clients.claim();
});

// Fetch: network-first for API, cache-first for static assets
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET requests
  if (request.method !== 'GET') return;

  // Skip cross-origin requests
  if (url.origin !== self.location.origin) return;

  // API calls: network-first, no cache
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(request).catch(() =>
        new Response(
          JSON.stringify({ error: { code: 'OFFLINE', message: 'You are offline' } }),
          { headers: { 'Content-Type': 'application/json' }, status: 503 }
        )
      )
    );
    return;
  }

  // Static assets: cache-first
  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;

      return fetch(request)
        .then((response) => {
          // Cache successful responses for static assets
          if (response.ok && (
            request.url.match(/\.(js|css|png|jpg|jpeg|svg|ico|woff|woff2)$/)
          )) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
          }
          return response;
        })
        .catch(() => {
          // Return offline page for navigation requests
          if (request.mode === 'navigate') {
            return caches.match('/offline');
          }
        });
    })
  );
});

// Push notifications
self.addEventListener('push', (event) => {
  if (!event.data) return;

  let data = {};
  try {
    data = event.data.json();
  } catch {
    data = { title: 'Notification', body: event.data.text() };
  }

  event.waitUntil(
    self.registration.showNotification(data.title || 'Notification', {
      body: data.body || '',
      icon: '/icons/icon-192x192.png',
      badge: '/icons/icon-96x96.png',
      data: data.url ? { url: data.url } : {},
      vibrate: [100, 50, 100],
    })
  );
});

// Notification click: open the app
self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url = event.notification.data?.url || '/';
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      for (const client of clientList) {
        if (client.url === url && 'focus' in client) {
          return client.focus();
        }
      }
      return clients.openWindow(url);
    })
  );
});
`
}

func pwaNextManifestTS(opts Options) string {
	name := strings.Title(strings.ReplaceAll(opts.ProjectName, "-", " "))
	return `import type { MetadataRoute } from 'next'

// Next.js App Router web manifest
// This file is served at /manifest.webmanifest automatically by Next.js.
// The public/manifest.json is the fallback for older browsers.

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: '` + name + `',
    short_name: '` + name + `',
    description: 'Built with Jua — Go + React for Africa',
    start_url: '/',
    display: 'standalone',
    background_color: '#0a0a0f',
    theme_color: '#6c5ce7',
    icons: [
      {
        src: '/icons/icon-192x192.png',
        sizes: '192x192',
        type: 'image/png',
        purpose: 'maskable',
      },
      {
        src: '/icons/icon-512x512.png',
        sizes: '512x512',
        type: 'image/png',
        purpose: 'maskable',
      },
    ],
  }
}
`
}

func pwaInstallBannerTSX() string {
	return `'use client'

import { usePWA } from '@/hooks/usePWA'

/**
 * PWA Install Banner
 * Shows an "Add to Home Screen" prompt when the browser fires the
 * beforeinstallprompt event (Chrome/Edge on Android/desktop).
 *
 * iOS Safari shows its own prompt — we show a manual instruction instead.
 *
 * Usage: Add <InstallBanner /> to your layout.tsx
 */
export function InstallBanner() {
  const { isInstallable, isIOS, isInstalled, install, dismiss } = usePWA()

  if (isInstalled) return null

  if (isIOS) {
    return (
      <div className="fixed bottom-4 left-4 right-4 z-50 rounded-xl border border-[#2a2a3a] bg-[#111118] p-4 shadow-lg md:left-auto md:w-80">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-[#e8e8f0]">Install App</p>
            <p className="mt-1 text-xs text-[#9090a8]">
              Tap{' '}
              <span className="font-mono text-[#6c5ce7]">Share</span>
              {' → '}
              <span className="font-mono text-[#6c5ce7]">Add to Home Screen</span>
            </p>
          </div>
          <button
            onClick={dismiss}
            className="text-[#606078] hover:text-[#9090a8] transition-colors"
            aria-label="Dismiss"
          >
            ✕
          </button>
        </div>
      </div>
    )
  }

  if (!isInstallable) return null

  return (
    <div className="fixed bottom-4 left-4 right-4 z-50 rounded-xl border border-[#2a2a3a] bg-[#111118] p-4 shadow-lg md:left-auto md:w-80">
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1">
          <p className="text-sm font-medium text-[#e8e8f0]">Install App</p>
          <p className="mt-1 text-xs text-[#9090a8]">
            Add to your home screen for a better experience
          </p>
        </div>
        <button
          onClick={dismiss}
          className="text-[#606078] hover:text-[#9090a8] transition-colors"
          aria-label="Dismiss"
        >
          ✕
        </button>
      </div>
      <div className="mt-3 flex gap-2">
        <button
          onClick={install}
          className="flex-1 rounded-lg bg-[#6c5ce7] px-3 py-2 text-sm font-medium text-white hover:bg-[#7c6cf7] transition-colors"
        >
          Install
        </button>
        <button
          onClick={dismiss}
          className="rounded-lg border border-[#2a2a3a] px-3 py-2 text-sm text-[#9090a8] hover:bg-[#1a1a24] transition-colors"
        >
          Not now
        </button>
      </div>
    </div>
  )
}
`
}

func pwsUsePWAHook() string {
	return `'use client'

import { useEffect, useState, useCallback } from 'react'

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

interface PWAState {
  isInstallable: boolean
  isInstalled: boolean
  isIOS: boolean
  isOnline: boolean
  install: () => Promise<void>
  dismiss: () => void
}

const DISMISSED_KEY = 'pwa-install-dismissed'

/**
 * usePWA — hook for PWA install prompt and online/offline detection.
 *
 * @example
 * const { isInstallable, install, isOnline } = usePWA()
 */
export function usePWA(): PWAState {
  const [installPrompt, setInstallPrompt] = useState<BeforeInstallPromptEvent | null>(null)
  const [isInstalled, setIsInstalled] = useState(false)
  const [isDismissed, setIsDismissed] = useState(false)
  const [isOnline, setIsOnline] = useState(true)

  const isIOS =
    typeof window !== 'undefined' &&
    /iPad|iPhone|iPod/.test(navigator.userAgent) &&
    !(window as unknown as { MSStream?: unknown }).MSStream

  useEffect(() => {
    // Check if already installed
    if (window.matchMedia('(display-mode: standalone)').matches) {
      setIsInstalled(true)
    }

    // Check if dismissed
    if (sessionStorage.getItem(DISMISSED_KEY)) {
      setIsDismissed(true)
    }

    // Listen for install prompt
    const handleBeforeInstall = (e: Event) => {
      e.preventDefault()
      setInstallPrompt(e as BeforeInstallPromptEvent)
    }
    window.addEventListener('beforeinstallprompt', handleBeforeInstall)

    // Listen for app installed
    const handleInstalled = () => setIsInstalled(true)
    window.addEventListener('appinstalled', handleInstalled)

    // Online/offline detection
    setIsOnline(navigator.onLine)
    const handleOnline = () => setIsOnline(true)
    const handleOffline = () => setIsOnline(false)
    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)

    // Register service worker
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker
        .register('/sw.js')
        .catch((err) => console.warn('SW registration failed:', err))
    }

    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstall)
      window.removeEventListener('appinstalled', handleInstalled)
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
    }
  }, [])

  const install = useCallback(async () => {
    if (!installPrompt) return
    await installPrompt.prompt()
    const { outcome } = await installPrompt.userChoice
    if (outcome === 'accepted') {
      setIsInstalled(true)
    }
    setInstallPrompt(null)
  }, [installPrompt])

  const dismiss = useCallback(() => {
    sessionStorage.setItem(DISMISSED_KEY, '1')
    setIsDismissed(true)
  }, [])

  return {
    isInstallable: !!installPrompt && !isDismissed,
    isInstalled,
    isIOS: isIOS && !isInstalled && !isDismissed,
    isOnline,
    install,
    dismiss,
  }
}
`
}
