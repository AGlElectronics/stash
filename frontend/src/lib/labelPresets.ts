/**
 * Dymo and generic label presets. Sizes are listed width × height in mm and
 * match the long edge being the wider dimension (which is how Dymo lists
 * "address" labels). The `landscape` flag tells us whether the printable
 * surface is wider than it is tall — affects barcode orientation.
 */
export type LabelPreset = {
  id: string;
  name: string;
  /** Printable area width in millimetres. */
  widthMm: number;
  /** Printable area height in millimetres. */
  heightMm: number;
  /** True when the label is wider than tall (most Dymo address labels). */
  landscape: boolean;
  /** Default Code 128 module width (mm) tuned for this label size. */
  defaultModuleWidthMm: number;
};

export const LABEL_PRESETS: readonly LabelPreset[] = [
  { id: 'dymo-30334', name: 'Dymo 30334 — Multi purpose (32 × 57 mm)', widthMm: 57, heightMm: 32, landscape: true, defaultModuleWidthMm: 0.3 },
  { id: 'dymo-30252', name: 'Dymo 30252 — Address (28 × 89 mm)', widthMm: 89, heightMm: 28, landscape: true, defaultModuleWidthMm: 0.5 },
  { id: 'dymo-30330', name: 'Dymo 30330 — Return address (19 × 51 mm)', widthMm: 51, heightMm: 19, landscape: true, defaultModuleWidthMm: 0.3 },
  { id: 'dymo-11354', name: 'Dymo 11354 — Multi purpose small (32 × 57 mm)', widthMm: 57, heightMm: 32, landscape: true, defaultModuleWidthMm: 0.3 },
  { id: 'dymo-30336', name: 'Dymo 30336 — Small multi purpose (25 × 54 mm)', widthMm: 54, heightMm: 25, landscape: true, defaultModuleWidthMm: 0.4 },
];

export const DEFAULT_PRESET_ID = 'dymo-30334';

/** Per-browser label size preference (localStorage on the print page, not admin Settings). */
export const LABEL_PRESET_STORAGE_KEY = 'stash.labelPreset';

export function findPreset(id: string): LabelPreset {
  const preset = LABEL_PRESETS.find((p) => p.id === id);
  if (preset) return preset;
  // Removed presets (square, A4 sheet) — fall back to default.
  return LABEL_PRESETS[0];
}
