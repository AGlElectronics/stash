import type { LabelPreset } from '@/lib/labelPresets';

/**
 * Print CSS for standalone label pages. Label size is chosen on the print page
 * (saved in localStorage per browser) — not in admin Settings.
 */
export function labelPrintCss(preset: LabelPreset): string {
  const { widthMm: w, heightMm: h, landscape } = preset;
  const pageSize = landscape ? `${w}mm ${h}mm landscape` : `${w}mm ${h}mm`;
  return `
    @media print {
      @page {
        size: ${pageSize};
        margin: 0;
      }
      html, body, #root {
        margin: 0 !important;
        padding: 0 !important;
        min-height: 0 !important;
        height: auto !important;
        width: auto !important;
        background: white !important;
        overflow: visible !important;
      }
      /* Screen chrome must not paginate — display:none, not visibility:hidden. */
      .print-hide {
        display: none !important;
      }
      .stash-print-sheet {
        display: block !important;
        position: static !important;
        margin: 0 !important;
        padding: 0 !important;
      }
      /* Only elements in the print sheet use this class (not the on-screen preview). */
      .stash-label {
        box-sizing: border-box;
        width: ${w}mm !important;
        height: ${h}mm !important;
        max-width: ${w}mm !important;
        max-height: ${h}mm !important;
        margin: 0 !important;
        padding: 0;
        border: none !important;
        box-shadow: none !important;
        overflow: hidden;
        page-break-inside: avoid;
        break-inside: avoid;
        -webkit-print-color-adjust: exact;
        print-color-adjust: exact;
      }
      .stash-label * {
        page-break-inside: avoid;
        break-inside: avoid;
      }
      /* Break between copies only — avoids trailing blank pages. */
      .stash-label + .stash-label {
        page-break-before: always;
        break-before: page;
      }
      .brand-label-mark {
        -webkit-print-color-adjust: exact;
        print-color-adjust: exact;
      }
    }
    @media screen {
      .stash-print-sheet {
        display: none !important;
      }
    }
  `;
}

export function LabelPrintStyles({ preset }: { preset: LabelPreset }) {
  return <style>{labelPrintCss(preset)}</style>;
}

/** On-screen note — hidden when printing (inside `.print-hide`). */
export function LabelPrinterNotice() {
  return (
    <aside className="w-full max-w-3xl rounded-lg border border-border bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
      <p className="font-medium text-foreground">Label printers only</p>
      <p className="mt-1">
        This page is formatted for dedicated label printers (Dymo, Brother, Zebra, etc.) with
        roll or die-cut labels. Regular office printers often ignore the custom page size and scale
        the layout — the preview may look wrong, and output may clip or spill off the label. Choose
        the matching preset above, then pick the same label size in your printer&apos;s dialog.
      </p>
    </aside>
  );
}
