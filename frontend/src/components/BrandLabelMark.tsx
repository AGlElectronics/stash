import { useBrandOptional } from '@/brand/BrandProvider';

type Props = {
  widthMm: number;
  heightMm: number;
};

/**
 * Org logo for label printing (standalone /brand.json or Den after pairing).
 * A plain grayscale image: label printers handled the earlier CSS-mask
 * version badly.
 */
export function BrandLabelMark({ widthMm, heightMm }: Props) {
  const { brand } = useBrandOptional();
  if (!brand.logoUrl) return null;
  return (
    <img
      src={brand.logoUrl}
      alt={brand.organizationName || brand.productName}
      className="brand-label-mark shrink-0"
      style={{
        width: `${widthMm}mm`,
        height: `${heightMm}mm`,
        objectFit: 'contain',
        filter: 'grayscale(1) contrast(1.4)',
      }}
    />
  );
}
