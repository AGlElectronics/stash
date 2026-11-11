/**
 * Neutralise spreadsheet formulas: text starting with = + - @ or a
 * tab/carriage return gets a leading apostrophe so Excel and LibreOffice
 * show it as text. Plain numbers are left alone. Mirrors csvSafe in the
 * backend.
 */
export function csvFormulaSafe(value: string): string {
  if (!value) return value;
  if (/^[=+\-@\t\r]/.test(value) && !/^[+-]?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i.test(value)) {
    return `'${value}`;
  }
  return value;
}

/** Escape a single CSV field (RFC-style, Excel-friendly, formula-safe). */
export function csvEscape(value: string): string {
  const s = csvFormulaSafe(String(value ?? ''));
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function rowsToCsv(rows: string[][]): string {
  return rows.map((row) => row.map(csvEscape).join(',')).join('\r\n');
}

/** Save a blob under the given filename. */
export function downloadBlob(filename: string, blob: Blob): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

/** UTF-8 with BOM so Excel recognizes UTF-8. */
export function downloadCsvFile(filename: string, csvBody: string): void {
  downloadBlob(filename, new Blob(['\ufeff', csvBody], { type: 'text/csv;charset=utf-8' }));
}
