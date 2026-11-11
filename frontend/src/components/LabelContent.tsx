import { Barcode } from '@/components/Barcode';
import { BrandLabelMark } from '@/components/BrandLabelMark';
import { QRCode } from '@/components/QRCode';
import type { LabelPreset } from '@/lib/labelPresets';

/** What carries the code: a 1D barcode, or a QR code (a link to /scan). */
export type LabelSymbology = 'barcode' | 'qr';

type Props = {
  code: string;
  /** Short line above the barcode (item name or L&F description). */
  title?: string;
  preset: LabelPreset;
  moduleWidthMm: number;
  forPrint?: boolean;
  symbology?: LabelSymbology;
  /** What the QR code encodes (usually scanUrl(code)); defaults to code. */
  qrValue?: string;
  /** Print the organisation's logo. */
  showLogo?: boolean;
};

/**
 * Printable label body. Sizes are in mm so output matches label printers.
 *
 * - barcode: optional logo, title, then a Code 128 barcode with its text;
 * - qr: a QR code on the left; logo, title and the code as text on the right.
 */
export function LabelContent({
  code,
  title,
  preset,
  moduleWidthMm,
  forPrint = false,
  symbology = 'barcode',
  qrValue,
  showLogo = true,
}: Props) {
  const padMm = Math.max(0.5, preset.heightMm * 0.05);
  const frame = `${forPrint ? 'stash-label' : 'stash-label-preview overflow-hidden'} bg-white text-black ${forPrint ? '' : 'rounded-md border border-border shadow-sm'}`;
  const box = {
    width: `${preset.widthMm}mm`,
    height: `${preset.heightMm}mm`,
    padding: `${padMm}mm`,
    boxSizing: 'border-box' as const,
  };

  if (!code.trim()) {
    return (
      <div className={`${frame} flex items-center justify-center`} style={box}>
        <span className="text-xs text-gray-500">No code yet</span>
      </div>
    );
  }

  const innerHm = preset.heightMm - padMm * 2;
  const titleFontMm = Math.max(2, preset.heightMm * 0.12);
  const titleEl = title ? (
    <div
      className="w-full shrink-0"
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
  ) : null;

  if (symbology === 'qr') {
    const value = qrValue || code;
    // Sized for a version-4 QR (33 modules, enough for a /scan link) plus a
    // 2-module quiet zone on each side; the label's own margin adds the rest.
    const cellMm = innerHm / 37;
    return (
      <div className={`${frame} flex items-center gap-[1mm] min-h-0`} style={box}>
        <QRCode value={value} cellSize={cellMm} quietZone={2} unit="mm" ecLevel="M" className="shrink-0" />
        <div className="flex min-w-0 flex-1 flex-col justify-center gap-[0.5mm]">
          {showLogo ? <BrandLabelMark widthMm={innerHm * 0.6} heightMm={innerHm * 0.25} /> : null}
          {titleEl}
          <div style={{ fontFamily: 'ui-monospace, monospace', fontSize: `${Math.max(1.8, titleFontMm * 0.85)}mm` }}>
            {code}
          </div>
        </div>
      </div>
    );
  }

  const logoHm = showLogo ? innerHm * 0.2 : 0;
  const barcodeHeightMm = Math.max(6, (innerHm - logoHm) * (title ? 0.5 : 0.65));
  return (
    <div className={`${frame} flex flex-col items-center justify-center text-center min-h-0`} style={box}>
      {showLogo ? <BrandLabelMark widthMm={preset.widthMm * 0.4} heightMm={logoHm} /> : null}
      {titleEl}
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
