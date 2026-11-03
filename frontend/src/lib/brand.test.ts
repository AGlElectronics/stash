import { describe, expect, it } from 'vitest';
import { DEFAULT_BRAND, normalizeBrand, withDenBrand } from './brand';

describe('normalizeBrand', () => {
  it('fills defaults for empty input', () => {
    const brand = normalizeBrand({}, 'standalone');
    expect(brand.productName).toBe(DEFAULT_BRAND.productName);
    expect(brand.source).toBe('standalone');
  });

  it('keeps provided organization and colors', () => {
    const brand = normalizeBrand(
      {
        organizationName: 'Example Con',
        colors: { primary: '#112233' },
      },
      'standalone'
    );
    expect(brand.organizationName).toBe('Example Con');
    expect(brand.colors.primary).toBe('#112233');
    expect(brand.colors.carbon).toBe(DEFAULT_BRAND.colors.carbon);
  });

  it('accepts den source', () => {
    const brand = normalizeBrand({ source: 'den' }, 'standalone');
    expect(brand.source).toBe('den');
  });
});

describe('withDenBrand', () => {
  it('keeps the standalone pack when not paired', () => {
    const pack = normalizeBrand({ organizationName: 'Local' });
    expect(withDenBrand(pack, undefined)).toBe(pack);
  });

  it('overlays organisation, logo and primary from Den', () => {
    const pack = normalizeBrand({ organizationName: 'Local' });
    const brand = withDenBrand(pack, {
      source: 'den', revision: 7, organization_name: 'Example Con', logo_url: '/api/brand/logo?v=1',
      theme: { primary: '#112233' }, copy: {},
    });
    expect(brand.source).toBe('den');
    expect(brand.organizationName).toBe('Example Con');
    expect(brand.logoUrl).toBe('/api/brand/logo?v=1');
    expect(brand.colors.primary).toBe('#112233');
    expect(brand.productName).toBe(pack.productName);
  });
});
