import { Info } from 'lucide-react';

/** Short volunteer-facing note: Code 128 barcodes, not QR; USB scanners work best. */
export function BarcodeHelpNote() {
  return (
    <div className="rounded-md border border-border bg-muted/40 p-3 text-sm text-muted-foreground space-y-1">
      <p className="flex items-center gap-2 font-medium text-foreground">
        <Info className="w-4 h-4 shrink-0" />
        How barcodes work
      </p>
      <ul className="list-disc pl-5 space-y-0.5">
        <li>Labels use <strong className="text-foreground font-medium">Code 128</strong> barcodes — not QR codes. Phone QR apps will not work.</li>
        <li>
          <strong className="text-foreground font-medium">USB barcode scanner</strong> (keyboard-wedge): plug into the desk PC, scan — the code types into the field automatically. No extra app needed.
        </li>
        <li>You can also use the camera button here, or type the code manually (e.g. <span className="font-mono">ST000042</span> or <span className="font-mono">LF-0003</span>).</li>
      </ul>
    </div>
  );
}
