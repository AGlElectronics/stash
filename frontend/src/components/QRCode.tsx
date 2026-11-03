import { useMemo } from 'react';
import qrcode from 'qrcode-generator';
import { cn } from '@/lib/utils';

type Unit = 'px' | 'mm';

type Props = {
  value: string;
  /** Module (cell) size. */
  cellSize?: number;
  /** Quiet zone width in modules (spec requires >= 4). */
  quietZone?: number;
  /** Unit for cellSize. Use 'mm' for print layouts. */
  unit?: Unit;
  /** Error correction level (L=7%, M=15%, Q=25%, H=30%). */
  ecLevel?: 'L' | 'M' | 'Q' | 'H';
  className?: string;
};

/**
 * Renders a QR code as inline SVG. Black modules on white background to
 * guarantee scanner/printer contrast in both light and dark UI themes.
 */
export function QRCode({
  value,
  cellSize = 4,
  quietZone = 4,
  unit = 'px',
  ecLevel = 'M',
  className,
}: Props) {
  const paths = useMemo(() => {
    if (!value) return null;
    try {
      const qr = qrcode(0, ecLevel);
      qr.addData(value);
      qr.make();
      const count = qr.getModuleCount();
      const rects: string[] = [];
      for (let row = 0; row < count; row++) {
        for (let col = 0; col < count; col++) {
          if (qr.isDark(row, col)) {
            const x = (col + quietZone) * cellSize;
            const y = (row + quietZone) * cellSize;
            rects.push(`M${x},${y}h${cellSize}v${cellSize}h-${cellSize}z`);
          }
        }
      }
      const totalSize = (count + quietZone * 2) * cellSize;
      return { d: rects.join(''), size: totalSize, modules: count };
    } catch {
      return null;
    }
  }, [value, cellSize, quietZone, ecLevel]);

  if (!paths) {
    return (
      <span role="alert" className={cn('text-xs text-destructive', className)}>
        Cannot render QR code
      </span>
    );
  }

  const dim = unit === 'mm' ? `${paths.size}mm` : paths.size;

  return (
    <svg
      role="img"
      aria-label={`QR code: ${value}`}
      viewBox={`0 0 ${paths.size} ${paths.size}`}
      width={dim}
      height={dim}
      shapeRendering="crispEdges"
      className={cn('bg-white', className)}
      style={{ display: 'block' }}
    >
      <rect x={0} y={0} width={paths.size} height={paths.size} fill="white" />
      <path d={paths.d} fill="black" />
    </svg>
  );
}
