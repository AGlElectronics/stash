import { describe, expect, it } from 'vitest';
import { encodeCode128B } from './barcode';

describe('encodeCode128B', () => {
  it('throws on empty input', () => {
    expect(() => encodeCode128B('')).toThrow();
  });

  it('throws on characters outside 32..127', () => {
    expect(() => encodeCode128B('A\u0000B')).toThrow();
    expect(() => encodeCode128B('A\u0080B')).toThrow();
  });

  it('encodes a simple asset tag', () => {
    const out = encodeCode128B('AW00042');
    // start + 7 chars + checksum + stop = 10 patterns × 11 modules + 2 (stop tail) = 112 modules
    const totalBars = out.widths.reduce((s, w) => s + w, 0);
    expect(totalBars).toBe(112);
    expect(out.totalModules).toBe(112 + 20);
    // Widths must alternate bar/space; the count must be reasonable (≤ each
    // pattern can produce up to 6 bars + 5 spaces).
    expect(out.widths.length).toBeGreaterThan(15);
    expect(out.widths.every((w) => w >= 1 && w <= 4)).toBe(true);
  });

  it('produces deterministic output for the same input', () => {
    const a = encodeCode128B('TEST-123');
    const b = encodeCode128B('TEST-123');
    expect(a.widths).toEqual(b.widths);
    expect(a.totalModules).toBe(b.totalModules);
  });

  it('encodes "Wikipedia" with the canonical checksum example', () => {
    // The Wikipedia Code 128 article uses "Wikipedia" as a worked example.
    // Spec values: W=55, i=73, k=75, i=73, p=80, e=69, d=68, i=73, a=65
    // Start B=104. Checksum positions are 1..n.
    // sum = 104
    //   + 55*1 + 73*2 + 75*3 + 73*4 + 80*5 + 69*6 + 68*7 + 73*8 + 65*9
    //   = 104 + 55 + 146 + 225 + 292 + 400 + 414 + 476 + 584 + 585
    //   = 3281; 3281 % 103 = 88
    // We can't read the checksum directly from EncodedBarcode, but we can
    // assert the total bar/space pattern length matches: 12 patterns × 11
    // (start + 9 chars + checksum + stop) + 2 (stop tail) = 134 modules.
    const out = encodeCode128B('Wikipedia');
    const totalBars = out.widths.reduce((s, w) => s + w, 0);
    expect(totalBars).toBe(134);
  });
});
