// Dev auth shim. When the backend runs with AUTH_MODE=dev it accepts a
// X-Dev-User header. The frontend sends the value of localStorage["stash.devUser"]
// or "dev-admin" by default. v0.4 will replace this with oidc-client-ts.

import { setAuthHeadersResolver } from '@/api/client';

const STORAGE_KEY = 'stash.devUser';

export function getDevUser(): string {
  if (typeof window === 'undefined') return 'dev-admin';
  return window.localStorage.getItem(STORAGE_KEY) ?? 'dev-admin';
}

export function setDevUser(name: string) {
  window.localStorage.setItem(STORAGE_KEY, name);
}

export function installDevAuth() {
  setAuthHeadersResolver(async () => ({
    'X-Dev-User': getDevUser(),
  }));
}
