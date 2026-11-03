import { useMemo } from 'react';
import { encodeCode128B } from '@/lib/barcode';
import { cn } from '@/lib/utils';

type Unit = 'px' | 'mm';

type Props = {
  value: string;
  /** Module (narrowest bar) width. */
  moduleWidth?: number;
  /** Bar height. */
  height?: number;
  /** Unit for moduleWidth/height. Use 'mm' when rendering inside a print
   *  layout so the SVG is sized to the printer rather than the screen. */
  unit?: Unit;
  /** Show the human-readable value below the bars. */
  showText?: boolean;
  className?: string;
};

/**
 * Renders a Code 128 (subset B) barcode as inline SVG. Uses CSS variables so
 * bars stay visible in both light and dark themes (we draw black on white,
 * since scanners and printers expect that contrast — even on dark UIs).
 */
export function Barcode({
  value,
  moduleWidth = 2,
  height = 60,
  unit = 'px',
  showText = true,
  className,
}: Props) {
  const encoded = useMemo(() => {
    try {
      return encodeCode128B(value);
    } catch (err) {
      return { widths: [], totalModules: 0, error: (err as Error).message } as const;
    }
  }, [value]);

  if ('error' in encoded) {
    return (
      <span role="alert" className={cn('text-xs text-destructive', className)}>
        Cannot render barcode: {encoded.error}
      </span>
    );
  }

  const quiet = 10;
  const totalWidth = encoded.totalModules * moduleWidth;
  const textHeight = showText ? Math.max(unit === 'mm' ? 2.5 : 16, height * 0.22) : 0;
  const totalHeight = height + textHeight;

  let cursor = quiet;
  const rects: Array<{ x: number; w: number }> = [];
  encoded.widths.forEach((w, idx) => {
    if (idx % 2 === 0) {
      rects.push({ x: cursor * moduleWidth, w: w * moduleWidth });
    }
    cursor += w;
  });

  // SVG dimensions are emitted with explicit unit suffixes so layout in a
  // mm-sized print page is faithful, while the on-screen px path keeps its
  // existing crisp pixel rendering.
  const w = unit === 'mm' ? `${totalWidth}mm` : totalWidth;
  const h = unit === 'mm' ? `${totalHeight}mm` : totalHeight;
  const fontSize = unit === 'mm' ? Math.max(2, height * 0.18) : 12;

  return (
    <div
      className={cn(
        'inline-flex flex-col items-center bg-white text-black rounded-md',
        unit === 'mm' ? 'gap-0 p-0' : 'gap-1 p-2',
        className
      )}
      data-testid="barcode"
    >
      <svg
        role="img"
        aria-label={`Barcode ${value}`}
        viewBox={`0 0 ${totalWidth} ${totalHeight}`}
        width={w}
        height={h}
        shapeRendering="crispEdges"
        style={{ display: 'block' }}
      >
        <rect x={0} y={0} width={totalWidth} height={totalHeight} fill="white" />
        {rects.map((r, i) => (
          <rect key={i} x={r.x} y={0} width={r.w} height={height} fill="black" />
        ))}
        {showText ? (
          <text
            x={totalWidth / 2}
            y={height + (textHeight * 0.78)}
            textAnchor="middle"
            fontFamily="ui-monospace, SFMono-Regular, Menlo, monospace"
            fontSize={fontSize}
            fill="black"
          >
            {value}
          </text>
        ) : null}
      </svg>
    </div>
  );
}
