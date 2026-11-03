import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import {
  applyBrandToDocument,
  DEFAULT_BRAND,
  loadDenBrand,
  loadStandaloneBrand,
  type BrandPack,
  type BrandSource,
} from '@/lib/brand';

type BrandContextValue = {
  brand: BrandPack;
  loading: boolean;
  error: string | null;
  /** Re-load standalone /brand.json or Den branding. */
  refresh: () => Promise<void>;
  /**
   * Switch brand source. `den` requires baseUrl + credential from pairing.
   * On failure, keeps previous brand and sets error.
   */
  setBrandSource: (
    source: BrandSource,
    opts?: { denBaseUrl?: string; credential?: string }
  ) => Promise<void>;
};

const BrandContext = createContext<BrandContextValue | null>(null);

const DEN_PAIRING_STORAGE_KEY = 'stash.denPairing';

type StoredDenPairing = { denBaseUrl: string; credential: string };

function readStoredDenPairing(): StoredDenPairing | null {
  try {
    const raw = localStorage.getItem(DEN_PAIRING_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as StoredDenPairing;
    if (
      typeof parsed?.denBaseUrl === 'string' &&
      parsed.denBaseUrl &&
      typeof parsed?.credential === 'string' &&
      parsed.credential
    ) {
      return parsed;
    }
  } catch {
    /* ignore */
  }
  return null;
}

function writeStoredDenPairing(pairing: StoredDenPairing | null): void {
  if (!pairing) {
    localStorage.removeItem(DEN_PAIRING_STORAGE_KEY);
    return;
  }
  localStorage.setItem(DEN_PAIRING_STORAGE_KEY, JSON.stringify(pairing));
}

export function BrandProvider({ children }: { children: ReactNode }) {
  const [brand, setBrand] = useState<BrandPack>(DEFAULT_BRAND);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const pairing = readStoredDenPairing();
      if (pairing) {
        try {
          const denBrand = await loadDenBrand(pairing.denBaseUrl, pairing.credential);
          applyBrandToDocument(denBrand);
          setBrand(denBrand);
          return;
        } catch (e) {
          // Den unreachable: fall back to standalone cache / local brand.
          setError(e instanceof Error ? e.message : 'Failed to load Den branding');
        }
      }
      const standalone = await loadStandaloneBrand();
      applyBrandToDocument(standalone);
      setBrand(standalone);
    } finally {
      setLoading(false);
    }
  }, []);

  const setBrandSource = useCallback(
    async (source: BrandSource, opts?: { denBaseUrl?: string; credential?: string }) => {
      setLoading(true);
      setError(null);
      try {
        if (source === 'den') {
          const denBaseUrl = opts?.denBaseUrl?.trim() ?? '';
          const credential = opts?.credential?.trim() ?? '';
          if (!denBaseUrl || !credential) {
            throw new Error('Den pairing requires base URL and credential');
          }
          const denBrand = await loadDenBrand(denBaseUrl, credential);
          writeStoredDenPairing({ denBaseUrl, credential });
          applyBrandToDocument(denBrand);
          setBrand(denBrand);
          return;
        }
        writeStoredDenPairing(null);
        const standalone = await loadStandaloneBrand();
        applyBrandToDocument(standalone);
        setBrand(standalone);
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Brand source switch failed');
        throw e;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const value = useMemo(
    () => ({ brand, loading, error, refresh, setBrandSource }),
    [brand, loading, error, refresh, setBrandSource]
  );

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
  const ctx = useContext(BrandContext);
  if (ctx) return ctx;
  return {
    brand: DEFAULT_BRAND,
    loading: false,
    error: null,
    refresh: async () => undefined,
    setBrandSource: async () => undefined,
  };
}
