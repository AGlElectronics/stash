import { describe, expect, it } from 'vitest';
import {
  extractScanCode,
  isLostFoundReportNumber,
  normalizeLostFoundReportNumber,
  scanUrl,
} from './scanCode';

describe('extractScanCode', () => {
  it('returns plain codes unchanged (trimmed)', () => {
    expect(extractScanCode(' ST000042 ')).toBe('ST000042');
  });

  it('reads the code from a QR label link', () => {
    expect(extractScanCode('https://stash.example.org/scan?code=ST000042')).toBe('ST000042');
    expect(extractScanCode('http://localhost:3000/scan/?code=LF-0003')).toBe('LF-0003');
  });

  it('round-trips scanUrl', () => {
    expect(extractScanCode(scanUrl('A B/1', 'https://x.test'))).toBe('A B/1');
  });

  it('leaves other links alone', () => {
    expect(extractScanCode('https://example.org/items?code=1')).toBe('https://example.org/items?code=1');
  });
});

describe('Lost & Found report numbers', () => {
  it('recognises both spellings', () => {
    expect(isLostFoundReportNumber('LF-0003')).toBe(true);
    expect(isLostFoundReportNumber('lf3')).toBe(true);
    expect(isLostFoundReportNumber('ST000003')).toBe(false);
  });

  it('normalises to LF-NNNN', () => {
    expect(normalizeLostFoundReportNumber('lf3')).toBe('LF-0003');
    expect(normalizeLostFoundReportNumber('LF-12345')).toBe('LF-12345');
  });
});
