import '@/auth/captureCallbackUrl'; // must run before router to preserve OIDC params

import { loadRuntimeConfig, renderStartupError } from '@/runtime-config';

if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register('/sw.js')
      .then((reg) => {
        // When a new SW is found, ask it to skip waiting so the next
        // navigation uses the fresh build. Without this, phones see a
        // 1-deploy lag (or worse, get stuck on a stale shell) because
        // the old SW keeps controlling the page until every tab is
        // closed.
        const askToSkip = (worker: ServiceWorker) => {
          worker.postMessage({ type: 'SKIP_WAITING' });
        };
        if (reg.waiting) askToSkip(reg.waiting);
        reg.addEventListener('updatefound', () => {
          const next = reg.installing;
          if (!next) return;
          next.addEventListener('statechange', () => {
            if (next.state === 'installed' && navigator.serviceWorker.controller) {
              askToSkip(next);
            }
          });
        });
      })
      .catch(() => undefined);

    // Reload once when the controller changes so the page re-fetches
    // /index.html via the new SW.
    let reloaded = false;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (reloaded) return;
      reloaded = true;
      window.location.reload();
    });
  });
}

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('#root element missing from index.html');
}

// The app is imported only after /config.json is in, so nothing (OIDC
// client, API client, router) can read a setting before it is known.
loadRuntimeConfig()
  .then(() => import('./app'))
  .then(({ startApp }) => startApp(rootEl))
  .catch((err: unknown) => renderStartupError(rootEl, err));
