import { useBrandOptional } from '@/brand/BrandProvider';

type Props = {
  widthMm: number;
  heightMm: number;
};

/**
 * Org mark for label printing. Logo URL and fill colour come from the active
 * brand pack (standalone /brand.json or Den after pairing).
 */
export function BrandLabelMark({ widthMm, heightMm }: Props) {
  const { brand } = useBrandOptional();
  const maskUrl = `url(${brand.logoUrl})`;
  return (
    <div
      className="brand-label-mark awoo-label-mark shrink-0"
      role="img"
      aria-label={brand.organizationName || brand.productName}
      style={{
        width: `${widthMm}mm`,
        height: `${heightMm}mm`,
        backgroundColor: brand.labelMarkColor,
        WebkitMaskImage: maskUrl,
        maskImage: maskUrl,
        WebkitMaskSize: 'contain',
        maskSize: 'contain',
        WebkitMaskRepeat: 'no-repeat',
        maskRepeat: 'no-repeat',
        WebkitMaskPosition: 'center',
        maskPosition: 'center',
        maskMode: 'luminance',
      }}
    />
  );
}

/** @deprecated Use BrandLabelMark — kept for existing imports. */
export const AwooLabelMark = BrandLabelMark;
