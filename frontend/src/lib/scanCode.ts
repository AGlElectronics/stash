/**
 * QR labels encode a link to the scan page (https://stash.example.org/scan?code=ST000042)
 * so a phone's own camera app opens Stash. Scanners inside Stash read the
 * same label: this returns the code from such a link, or the text itself.
 */
export function extractScanCode(text: string): string {
  const trimmed = text.trim();
  if (!/^https?:\/\//i.test(trimmed)) return trimmed;
  try {
    const url = new URL(trimmed);
    const code = url.searchParams.get('code');
    if (url.pathname.replace(/\/+$/, '').endsWith('/scan') && code) return code.trim();
  } catch {
    // not a URL after all
  }
  return trimmed;
}

/** The link a QR label carries for a code. */
export function scanUrl(code: string, origin = window.location.origin): string {
  return `${origin}/scan?code=${encodeURIComponent(code)}`;
}

/** Report numbers printed on found-item labels, e.g. LF-0003 or LF0003. */
export function isLostFoundReportNumber(code: string): boolean {
  return /^LF-?\d+$/i.test(code.trim());
}

/** "lf0003" → "LF-0003". */
export function normalizeLostFoundReportNumber(code: string): string {
  const digits = code.trim().replace(/^LF-?/i, '');
  return `LF-${digits.padStart(4, '0')}`;
}
