import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { applyBrandToDocument, DEFAULT_BRAND, loadStandaloneBrand, withDenBrand, type BrandPack } from '@/lib/brand';
import { useModuleBrand } from '@/lib/moduleBrand';

type BrandContextValue = {
  brand: BrandPack;
  loading: boolean;
};

const BrandContext = createContext<BrandContextValue | null>(null);

/**
 * Brand = the deployment's /brand.json (product name, copy) overlaid with
 * Den's brand when the backend is paired (organisation name, logo, colours).
 * Den changes show up within a minute, no rebuild needed.
 */
export function BrandProvider({ children }: { children: ReactNode }) {
  const [standalone, setStandalone] = useState<BrandPack | null>(null);
  const den = useModuleBrand('/api/brand', null);

  useEffect(() => {
    let cancelled = false;
    void loadStandaloneBrand().then((b) => {
      if (!cancelled) setStandalone(b);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const brand = useMemo(() => withDenBrand(standalone ?? DEFAULT_BRAND, den), [standalone, den]);
  useEffect(() => applyBrandToDocument(brand), [brand]);

  const value = useMemo(() => ({ brand, loading: standalone === null }), [brand, standalone]);
  return <BrandContext.Provider value={value}>{children}</BrandContext.Provider>;
}

export function useBrand(): BrandContextValue {
  const ctx = useContext(BrandContext);
  if (!ctx) {
    throw new Error('useBrand must be used within BrandProvider');
  }
  return ctx;
}

/** Safe hook for optional usage outside provider (tests) — returns defaults. */
export function useBrandOptional(): BrandContextValue {
  return useContext(BrandContext) ?? { brand: DEFAULT_BRAND, loading: false };
}
