/**
 * Brand pack for Stash.
 *
 * - standalone: loaded from /brand.json (local theme / deployment overlay)
 * - den: the backend pairs with Den and serves Den's brand at /api/brand
 *   (logo proxied by the backend); see withDenBrand.
 *
 * Never give another module credentials to Stash's bucket, and never read
 * Den's bucket directly from Stash.
 */
import type { ModuleBrand } from '@/lib/moduleBrand';

export type BrandSource = 'standalone' | 'den';

export type BrandColors = {
  primary: string;
  carbon: string;
  shadow: string;
};

export type BrandPack = {
  revision: number;
  source: BrandSource;
  productName: string;
  organizationName: string;
  loginSubtitle: string;
  loginAccountHint: string;
  loginButtonLabel: string;
  /** Human-facing desk name on public claim copy (e.g. "operations desk"). */
  publicDeskName: string;
  claimHeaderSubtitle: string;
  logoUrl: string;
  /** Picture behind the login screen ("" = none), "panel" | "centered", caption. */
  loginBackgroundUrl: string;
  loginLayout: string;
  loginCaption: string;
  /** Label mark fill; empty means the primary colour. */
  labelMarkColor: string;
  themeColor: string;
  colors: BrandColors;
  /** Optional CSS custom properties applied to :root (e.g. "--primary": "243 75% 59%"). */
  cssVariables?: Record<string, string>;
};

export const DEFAULT_BRAND: BrandPack = {
  revision: 0,
  source: 'standalone',
  productName: 'Stash',
  organizationName: '',
  loginSubtitle: 'Inventory & lost property',
  loginAccountHint:
    "Sign in with your organisation account — we'll send you to the identity provider and bring you right back.",
  loginButtonLabel: 'Continue with organisation login',
  publicDeskName: 'operations desk',
  claimHeaderSubtitle: 'Claim your belongings',
  logoUrl: '/logo.svg',
  loginBackgroundUrl: '',
  loginLayout: 'panel',
  loginCaption: '',
  labelMarkColor: '',
  themeColor: '#18181b',
  colors: {
    primary: '#5048e5',
    carbon: '#18181b',
    shadow: '#241e20',
  },
  cssVariables: {},
};

function asString(v: unknown, fallback: string): string {
  return typeof v === 'string' && v.trim() !== '' ? v : fallback;
}

function asNumber(v: unknown, fallback: number): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : fallback;
}

/** Normalize partial JSON into a full BrandPack. */
export function normalizeBrand(raw: unknown, source: BrandSource = 'standalone'): BrandPack {
  const o = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {};
  const colorsRaw =
    o.colors && typeof o.colors === 'object' ? (o.colors as Record<string, unknown>) : {};
  const cssVariables =
    o.cssVariables && typeof o.cssVariables === 'object'
      ? Object.fromEntries(
          Object.entries(o.cssVariables as Record<string, unknown>).filter(
            ([, v]) => typeof v === 'string'
          ) as [string, string][]
        )
      : {};

  const sourceField = o.source === 'den' || o.source === 'standalone' ? o.source : source;

  return {
    revision: asNumber(o.revision, DEFAULT_BRAND.revision),
    source: sourceField,
    productName: asString(o.productName, DEFAULT_BRAND.productName),
    organizationName: asString(o.organizationName, DEFAULT_BRAND.organizationName),
    loginSubtitle: asString(o.loginSubtitle, DEFAULT_BRAND.loginSubtitle),
    loginAccountHint: asString(o.loginAccountHint, DEFAULT_BRAND.loginAccountHint),
    loginButtonLabel: asString(o.loginButtonLabel, DEFAULT_BRAND.loginButtonLabel),
    publicDeskName: asString(o.publicDeskName, DEFAULT_BRAND.publicDeskName),
    claimHeaderSubtitle: asString(o.claimHeaderSubtitle, DEFAULT_BRAND.claimHeaderSubtitle),
    logoUrl: asString(o.logoUrl, DEFAULT_BRAND.logoUrl),
    loginBackgroundUrl: asString(o.loginBackgroundUrl, DEFAULT_BRAND.loginBackgroundUrl),
    loginLayout: asString(o.loginLayout, DEFAULT_BRAND.loginLayout),
    loginCaption: asString(o.loginCaption, DEFAULT_BRAND.loginCaption),
    labelMarkColor: asString(o.labelMarkColor, DEFAULT_BRAND.labelMarkColor),
    themeColor: asString(o.themeColor, DEFAULT_BRAND.themeColor),
    colors: {
      primary: asString(colorsRaw.primary, DEFAULT_BRAND.colors.primary),
      carbon: asString(colorsRaw.carbon, DEFAULT_BRAND.colors.carbon),
      shadow: asString(colorsRaw.shadow, DEFAULT_BRAND.colors.shadow),
    },
    cssVariables,
  };
}

/** Overlays Den's brand (organisation name, logo, primary colour) when paired. */
export function withDenBrand(pack: BrandPack, den: ModuleBrand | undefined): BrandPack {
  if (!den || den.source !== 'den') return pack;
  const primary = typeof den.theme?.primary === 'string' ? den.theme.primary : pack.colors.primary;
  return {
    ...pack,
    source: 'den',
    revision: den.revision,
    organizationName: den.organization_name || pack.organizationName,
    logoUrl: den.logo_url || pack.logoUrl,
    loginBackgroundUrl: den.login_background_url || pack.loginBackgroundUrl,
    loginLayout: asString(den.copy?.login_layout, pack.loginLayout),
    loginCaption: asString(den.copy?.login_caption, pack.loginCaption),
    colors: { ...pack.colors, primary },
  };
}

/** Apply brand tokens to document (title, theme-color, optional CSS variables). */
export function applyBrandToDocument(brand: BrandPack): void {
  if (typeof document === 'undefined') return;

  const title =
    brand.organizationName.trim() !== ''
      ? `${brand.productName} — ${brand.organizationName}`
      : brand.productName;
  document.title = title;

  let themeMeta = document.querySelector('meta[name="theme-color"]');
  if (!themeMeta) {
    themeMeta = document.createElement('meta');
    themeMeta.setAttribute('name', 'theme-color');
    document.head.appendChild(themeMeta);
  }
  themeMeta.setAttribute('content', brand.themeColor);

  const root = document.documentElement;
  root.style.setProperty('--brand-primary', brand.colors.primary);
  root.style.setProperty('--brand-carbon', brand.colors.carbon);
  root.style.setProperty('--brand-shadow', brand.colors.shadow);

  for (const [key, value] of Object.entries(brand.cssVariables ?? {})) {
    const name = key.startsWith('--') ? key : `--${key}`;
    root.style.setProperty(name, value);
  }
}

/**
 * Load standalone brand from /brand.json.
 * Falls back to DEFAULT_BRAND if missing or invalid.
 */
export async function loadStandaloneBrand(signal?: AbortSignal): Promise<BrandPack> {
  try {
    const res = await fetch('/brand.json', { signal, cache: 'no-cache' });
    if (!res.ok) return { ...DEFAULT_BRAND };
    const json: unknown = await res.json();
    return normalizeBrand(json, 'standalone');
  } catch {
    return { ...DEFAULT_BRAND };
  }
}
