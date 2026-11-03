import { describe, expect, it } from 'vitest';
import { DEFAULT_BRAND, normalizeBrand } from './brand';

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
    const brand = normalizeBrand({ source: 'den', denBaseUrl: 'http://den.local' }, 'standalone');
    expect(brand.source).toBe('den');
    expect(brand.denBaseUrl).toBe('http://den.local');
  });
});
