import { useState } from 'react';
import { useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { Printer } from 'lucide-react';
import { getItem } from '@/api/items';
import { LabelContent, type LabelSymbology } from '@/components/LabelContent';
import { scanUrl } from '@/lib/scanCode';
import { Button } from '@/components/ui/Button';
import { Field, Input, Select } from '@/components/ui/Input';
import { useLabelPrintPreset } from '@/hooks/useLabelPrintPreset';
import { LABEL_PRESETS } from '@/lib/labelPresets';
import { LabelPrintStyles, LabelPrinterNotice } from '@/lib/labelPrint';

/**
 * Live label print page.
 *
 * - Pick a label preset; the preview re-flows immediately so what you see
 *   is what your printer gets.
 * - Pick a barcode source (asset_tag → qr_code → serial_number, or any
 *   custom value typed in).
 * - Tweak module density (bar width) and how many copies to print.
 * - Sets @page size dynamically via an injected <style>, so any Dymo size
 *   the user selects gets a correct print sheet.
 */
export function ItemLabelPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const item = useQuery({ queryKey: ['item', id], queryFn: () => getItem(id) });

  const { presetId, setPresetId, preset, moduleWidth, setModuleWidth } = useLabelPrintPreset();
  const [copies, setCopies] = useState(1);
  const [showName, setShowName] = useState(true);
  const [showLogo, setShowLogo] = useState(true);
  const [symbology, setSymbology] = useState<LabelSymbology>('barcode');
  const [customCode, setCustomCode] = useState('');

  const autoCode = item.data?.asset_tag || item.data?.qr_code || item.data?.serial_number || '';
  const code = customCode.trim() || autoCode;
  const labelProps = {
    code,
    title: showName ? item.data?.name : undefined,
    preset,
    moduleWidthMm: moduleWidth,
    symbology,
    qrValue: code ? scanUrl(code) : '',
    showLogo,
  };

  if (item.isLoading) {
    return <div className="p-4 text-sm text-muted-foreground">Loading…</div>;
  }
  if (item.isError || !item.data) {
    return <div className="p-4 text-sm text-destructive">Failed to load item.</div>;
  }

  return (
    <>
      <LabelPrintStyles preset={preset} />
      <div className="min-h-screen bg-background flex flex-col items-center gap-6 p-6 print-hide">
        <header className="w-full max-w-3xl flex items-start justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold">Print label — {item.data.name}</h1>
            <p className="text-sm text-muted-foreground">
              Live preview. The print dialog uses the preset size automatically.
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
            <Field label="Override code (optional)" htmlFor="code" hint={autoCode ? `Default: ${autoCode}` : 'No barcode on this item yet.'}>
              <Input
                id="code"
                value={customCode}
                onChange={(e) => setCustomCode(e.target.value)}
                placeholder={autoCode}
                className="font-mono"
              />
            </Field>
            <Field
              label="Code style"
              htmlFor="symbology"
              hint="A QR label opens this item when scanned with any phone camera."
            >
              <Select
                id="symbology"
                value={symbology}
                onChange={(e) => setSymbology(e.target.value as LabelSymbology)}
              >
                <option value="barcode">Barcode (USB scanners)</option>
                <option value="qr">QR code (phones and USB scanners)</option>
              </Select>
            </Field>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={showName} onChange={(e) => setShowName(e.target.checked)} />
              Print item name
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={showLogo} onChange={(e) => setShowLogo(e.target.checked)} />
              Print the organisation logo
            </label>
          </div>

          <div className="space-y-2">
            <p className="text-xs uppercase text-muted-foreground tracking-wide">Preview ({preset.widthMm} × {preset.heightMm} mm)</p>
            <LabelContent {...labelProps} />
            {!code ? (
              <p className="text-sm text-warning">
                This item has no asset tag, QR code, or serial number. Type one above, or assign one from
                the item page first.
              </p>
            ) : null}
          </div>
        </div>

        <LabelPrinterNotice />
      </div>

      <div className="stash-print-sheet">
        {Array.from({ length: copies }).map((_, idx) => (
          <LabelContent key={idx} {...labelProps} forPrint />
        ))}
      </div>
    </>
  );
}
