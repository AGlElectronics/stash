/** Escape a single CSV field (RFC-style, Excel-friendly). */
export function csvEscape(value: string): string {
  const s = String(value ?? '');
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function rowsToCsv(rows: string[][]): string {
  return rows.map((row) => row.map(csvEscape).join(',')).join('\r\n');
}

/** UTF-8 with BOM so Excel recognizes UTF-8. */
export function downloadCsvFile(filename: string, csvBody: string): void {
  const blob = new Blob(['\ufeff', csvBody], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
