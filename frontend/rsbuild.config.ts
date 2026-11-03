import { defineConfig } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';

// Rsbuild does not auto-inject process.env into import.meta.env like Vite; Docker / CI must
// set VITE_* on the environment, and we map them here so production OIDC builds work.
const viteEnv = {
  VITE_AUTH_MODE: process.env.VITE_AUTH_MODE ?? 'dev',
  VITE_OIDC_AUTHORITY: process.env.VITE_OIDC_AUTHORITY ?? '',
  VITE_OIDC_CLIENT_ID: process.env.VITE_OIDC_CLIENT_ID ?? '',
  VITE_OIDC_SCOPE: process.env.VITE_OIDC_SCOPE ?? 'openid profile email',
  VITE_API_BASE_URL: process.env.VITE_API_BASE_URL ?? '',
  VITE_PUBLIC_CLAIM_ENABLED: process.env.VITE_PUBLIC_CLAIM_ENABLED ?? 'true',
} as const;

export default defineConfig({
  plugins: [pluginReact()],
  source: {
    entry: { index: './src/main.tsx' },
    define: {
      'process.env.STASH_VERSION': JSON.stringify(process.env.STASH_VERSION ?? 'dev'),
      'import.meta.env.VITE_AUTH_MODE': JSON.stringify(viteEnv.VITE_AUTH_MODE),
      'import.meta.env.VITE_OIDC_AUTHORITY': JSON.stringify(viteEnv.VITE_OIDC_AUTHORITY),
      'import.meta.env.VITE_OIDC_CLIENT_ID': JSON.stringify(viteEnv.VITE_OIDC_CLIENT_ID),
      'import.meta.env.VITE_OIDC_SCOPE': JSON.stringify(viteEnv.VITE_OIDC_SCOPE),
      'import.meta.env.VITE_API_BASE_URL': JSON.stringify(viteEnv.VITE_API_BASE_URL),
      'import.meta.env.VITE_PUBLIC_CLAIM_ENABLED': JSON.stringify(viteEnv.VITE_PUBLIC_CLAIM_ENABLED),
    },
  },
  resolve: {
    alias: {
      '@': './src',
    },
  },
  html: {
    template: './public/index.html',
    // Runtime brand.json / Den branding owns the visible title after boot.
    title: 'Stash',
    meta: {
      description: 'Inventory, sign-out and lost & found management',
      'theme-color': '#18181b',
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: process.env.API_PROXY_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  output: {
    distPath: { root: 'dist' },
    cleanDistPath: true,
    copy: [
      { from: 'public/manifest.webmanifest', to: 'manifest.webmanifest' },
      { from: 'public/brand.json', to: 'brand.json' },
      { from: 'public/sw.js', to: 'sw.js' },
      { from: 'public/icon-192.png', to: 'icon-192.png', noErrorOnMissing: true },
      { from: 'public/logo.svg', to: 'logo.svg', noErrorOnMissing: true },
      { from: 'public/icon-512.png', to: 'icon-512.png', noErrorOnMissing: true },
    ],
  },
});
