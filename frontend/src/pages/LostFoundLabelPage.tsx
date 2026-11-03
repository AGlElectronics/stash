import { useState } from 'react';
import { useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { Printer } from 'lucide-react';
import { getLostFoundReport } from '@/api/lostfound';
import { LabelContent } from '@/components/LabelContent';
import { BarcodeHelpNote } from '@/components/BarcodeHelpNote';
import { Button } from '@/components/ui/Button';
import { Field, Input, Select } from '@/components/ui/Input';
import { useLabelPrintPreset } from '@/hooks/useLabelPrintPreset';
import { LABEL_PRESETS } from '@/lib/labelPresets';
import { LabelPrintStyles, LabelPrinterNotice } from '@/lib/labelPrint';

/**
 * Print page for Lost & Found labels. Code 128 barcode encodes the report
 * number (LF-0003) so ConOps can scan a bag in the pile and jump straight
 * to that report.
 */
export function LostFoundLabelPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const report = useQuery({ queryKey: ['lostfound', id], queryFn: () => getLostFoundReport(id) });

  const { presetId, setPresetId, preset, moduleWidth, setModuleWidth } = useLabelPrintPreset();
  const [copies, setCopies] = useState(1);

  const code = report.data?.report_number ?? '';

  if (report.isLoading) {
    return <div className="p-4 text-sm text-muted-foreground">Loading…</div>;
  }
  if (report.isError || !report.data) {
    return <div className="p-4 text-sm text-destructive">Failed to load report.</div>;
  }

  const r = report.data;

  return (
    <>
      <LabelPrintStyles preset={preset} />
      <div className="min-h-screen bg-background flex flex-col items-center gap-6 p-6 print-hide">
        <header className="w-full max-w-3xl flex items-start justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold">Print label — {r.report_number}</h1>
            <p className="text-sm text-muted-foreground">
              {r.item_name} · Scan with Stash to open this report instantly.
            </p>
          </div>
          <div className="flex gap-2">
            <Button onClick={() => window.print()}>
              <Printer className="w-4 h-4" />
              Print
            </Button>
            <Button variant="outline" onClick={() => window.close()}>Close</Button>
          </div>
        </header>

        <div className="w-full max-w-3xl grid grid-cols-1 md:grid-cols-2 gap-6">
          <div className="space-y-3">
            <BarcodeHelpNote />
            <Field label="Label size" htmlFor="preset" hint="Saved in this browser only — not in admin Settings.">
              <Select id="preset" value={presetId} onChange={(e) => setPresetId(e.target.value)}>
                {LABEL_PRESETS.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </Select>
            </Field>
            <Field
              label={`Bar width (${moduleWidth.toFixed(2)} mm/module)`}
              htmlFor="module-width"
              hint={`Default for this label: ${preset.defaultModuleWidthMm.toFixed(2)} mm/module. Adjust if your printer needs it.`}
            >
              <input
                id="module-width"
                type="range"
                min={0.2}
                max={0.7}
                step={0.05}
                value={moduleWidth}
                onChange={(e) => setModuleWidth(Number(e.target.value))}
                className="w-full"
              />
            </Field>
            <Field label="Copies" htmlFor="copies">
              <Input
                id="copies"
                type="number"
                min={1}
                max={50}
                value={copies}
                onChange={(e) => setCopies(Math.max(1, Math.min(50, Number(e.target.value) || 1)))}
              />
            </Field>
          </div>

          <div className="space-y-2">
            <p className="text-xs uppercase text-muted-foreground tracking-wide">
              Preview ({preset.widthMm} × {preset.heightMm} mm)
            </p>
            <LabelContent
              code={code}
              title={r.item_name}
              preset={preset}
              moduleWidthMm={moduleWidth}
            />
          </div>
        </div>

        <LabelPrinterNotice />
      </div>

      <div className="stash-print-sheet">
        {Array.from({ length: copies }).map((_, idx) => (
          <LabelContent
            key={idx}
            code={code}
            title={r.item_name}
            preset={preset}
            moduleWidthMm={moduleWidth}
            forPrint
          />
        ))}
      </div>
    </>
  );
}
