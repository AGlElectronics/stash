/**
 * Brand pack for Stash.
 *
 * - standalone: loaded from /brand.json (local theme / deployment overlay)
 * - den: fetched from Den integration API after pairing (URL + token);
 *   media assets are requested via Den (Den reads its own S3 bucket).
 *
 * Never give another module credentials to Stash's bucket, and never read
 * Den's bucket directly from Stash.
 */

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
  labelMarkColor: string;
  themeColor: string;
  colors: BrandColors;
  /** Optional CSS custom properties applied to :root (e.g. "--primary": "349 74% 59%"). */
  cssVariables?: Record<string, string>;
  /** Present when source === 'den' — Den base URL used for media fetches. */
  denBaseUrl?: string;
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
  labelMarkColor: '#D93654',
  themeColor: '#18181b',
  colors: {
    primary: '#e44763',
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
    labelMarkColor: asString(o.labelMarkColor, DEFAULT_BRAND.labelMarkColor),
    themeColor: asString(o.themeColor, DEFAULT_BRAND.themeColor),
    colors: {
      primary: asString(colorsRaw.primary, DEFAULT_BRAND.colors.primary),
      carbon: asString(colorsRaw.carbon, DEFAULT_BRAND.colors.carbon),
      shadow: asString(colorsRaw.shadow, DEFAULT_BRAND.colors.shadow),
    },
    cssVariables,
    denBaseUrl: typeof o.denBaseUrl === 'string' ? o.denBaseUrl : undefined,
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

/**
 * Future: load brand metadata from Den using the pairing credential.
 * Media bytes must still be fetched via Den's media endpoint (Den owns the bucket).
 */
export async function loadDenBrand(
  denBaseUrl: string,
  credential: string,
  signal?: AbortSignal
): Promise<BrandPack> {
  const base = denBaseUrl.replace(/\/$/, '');
  const res = await fetch(`${base}/api/v1/integration/branding`, {
    signal,
    headers: { Authorization: `Bearer ${credential}`, Accept: 'application/json' },
  });
  if (!res.ok) {
    throw new Error(`Den branding fetch failed: HTTP ${res.status}`);
  }
  const json: unknown = await res.json();
  return normalizeBrand({ ...(json as object), denBaseUrl: base }, 'den');
}
