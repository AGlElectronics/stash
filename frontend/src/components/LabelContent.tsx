import { Barcode } from '@/components/Barcode';
import type { LabelPreset } from '@/lib/labelPresets';

type Props = {
  code: string;
  /** Short line above the barcode (item name or L&F description). */
  title?: string;
  preset: LabelPreset;
  moduleWidthMm: number;
  forPrint?: boolean;
};

/**
 * Printable label body: title + barcode, centered. (Logo temporarily removed.)
 * Sizes are in mm so output matches label printers.
 */
export function LabelContent({ code, title, preset, moduleWidthMm, forPrint = false }: Props) {
  const padMm = Math.max(0.5, preset.heightMm * 0.05);

  if (!code.trim()) {
    return (
      <div
        className={`${forPrint ? 'stash-label' : 'stash-label-preview overflow-hidden'} bg-white text-black ${forPrint ? '' : 'rounded-md border border-border shadow-sm'} flex items-center justify-center`}
        style={{
          width: `${preset.widthMm}mm`,
          height: `${preset.heightMm}mm`,
          padding: `${padMm}mm`,
          boxSizing: 'border-box',
        }}
      >
        <span className="text-xs text-gray-500">No code yet</span>
      </div>
    );
  }

  const innerHm = preset.heightMm - padMm * 2;
  const barcodeHeightMm = Math.max(6, innerHm * (title ? 0.5 : 0.65));
  const titleFontMm = Math.max(2, preset.heightMm * 0.12);

  // TEMP: logo removed from labels (printer struggles with the masked PNG).
  // Content is centered across the full label. Restore <AwooLabelMark/> later.
  return (
    <div
      className={`${forPrint ? 'stash-label' : 'stash-label-preview overflow-hidden'} bg-white text-black ${forPrint ? '' : 'rounded-md border border-border shadow-sm'} flex flex-col items-center justify-center min-h-0`}
      style={{
        width: `${preset.widthMm}mm`,
        height: `${preset.heightMm}mm`,
        padding: `${padMm}mm`,
        boxSizing: 'border-box',
      }}
    >
      {title ? (
        <div
          className="w-full shrink-0 text-center"
          style={{
            fontFamily: 'ui-sans-serif, system-ui, sans-serif',
            fontSize: `${titleFontMm}mm`,
            fontWeight: 600,
            lineHeight: 1.05,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={title}
        >
          {title}
        </div>
      ) : null}
      <div className="flex w-full shrink-0 items-center justify-center overflow-hidden">
        <Barcode
          value={code}
          moduleWidth={moduleWidthMm}
          height={barcodeHeightMm}
          unit="mm"
          showText
          className="bg-white max-w-full"
        />
      </div>
    </div>
  );
}
