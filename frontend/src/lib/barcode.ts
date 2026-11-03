// Minimal Code 128 (subset B) encoder. Subset B covers ASCII 32..127 which is
// what typical asset tags ("ST00042", "ABC-123", URLs, etc.) use. We do not
// implement subset C compaction (digit pairs) because label widths on a
// Dymo 30334 (32mm × 57mm) easily fit a 7-12 character tag rendered at the
// default module width.
//
// Each Code 128 character maps to an 11-module pattern (alternating bar/space
// starting with a bar). The stop pattern adds a final 2-module bar making it
// 13 modules wide.

// Indexed by Code 128 value (0..106). Each string is exactly 11 characters
// of '1' (bar) / '0' (space). Source: ISO/IEC 15417.
const PATTERNS: readonly string[] = [
  '11011001100', '11001101100', '11001100110', '10010011000',
  '10010001100', '10001001100', '10011001000', '10011000100',
  '10001100100', '11001001000', '11001000100', '11000100100',
  '10110011100', '10011011100', '10011001110', '10111001100',
  '10011101100', '10011100110', '11001110010', '11001011100',
  '11001001110', '11011100100', '11001110100', '11101101110',
  '11101001100', '11100101100', '11100100110', '11101100100',
  '11100110100', '11100110010', '11011011000', '11011000110',
  '11000110110', '10100011000', '10001011000', '10001000110',
  '10110001000', '10001101000', '10001100010', '11010001000',
  '11000101000', '11000100010', '10110111000', '10110001110',
  '10001101110', '10111011000', '10111000110', '10001110110',
  '11101110110', '11010001110', '11000101110', '11011101000',
  '11011100010', '11011101110', '11101011000', '11101000110',
  '11100010110', '11101101000', '11101100010', '11100011010',
  '11101111010', '11001000010', '11110001010', '10100110000',
  '10100001100', '10010110000', '10010000110', '10000101100',
  '10000100110', '10110010000', '10110000100', '10011010000',
  '10011000010', '10000110100', '10000110010', '11000010010',
  '11001010000', '11110111010', '11000010100', '10001111010',
  '10100111100', '10010111100', '10010011110', '10111100100',
  '10011110100', '10011110010', '11110100100', '11110010100',
  '11110010010', '11011011110', '11011110110', '11110110110',
  '10101111000', '10100011110', '10001011110', '10111101000',
  '10111100010', '11110101000', '11110100010', '10111011110',
  '10111101110', '11101011110', '11110101110', '11010000100',
  '11010010000', '11010011100', '11000111010',
];

const START_B = 104;
const STOP = 106;

export type EncodedBarcode = {
  /** Sequence of bar/space widths (in modules) starting with a bar. */
  widths: readonly number[];
  /** Total width in modules including quiet zones (>= 10 modules each side). */
  totalModules: number;
};

/**
 * Encodes a Code 128 (subset B) barcode. Throws if any character is outside
 * the 32..127 range (other characters require subset A or escapes that we
 * deliberately don't support yet).
 */
export function encodeCode128B(input: string): EncodedBarcode {
  if (!input) {
    throw new Error('barcode: empty input');
  }
  const values: number[] = [START_B];
  for (let idx = 0; idx < input.length; idx++) {
    const code = input.charCodeAt(idx);
    if (code < 32 || code > 127) {
      throw new Error(`barcode: character "${input[idx]}" is not encodable in Code 128 subset B`);
    }
    values.push(code - 32);
  }

  // Checksum: (start + Σ position * value) mod 103.
  // The start char itself contributes once (position 1 multiplier is 1).
  let sum = START_B;
  for (let i = 1; i < values.length; i++) {
    sum += values[i] * i;
  }
  const checksum = sum % 103;
  values.push(checksum);
  values.push(STOP);

  // Convert to a 0/1 string then collapse runs to widths.
  let bits = '';
  for (const v of values) {
    bits += PATTERNS[v];
  }
  // Stop pattern has an extra 2-module bar appended after the 11 modules.
  bits += '11';

  const widths: number[] = [];
  let runChar = bits[0];
  let runLen = 1;
  for (let i = 1; i < bits.length; i++) {
    if (bits[i] === runChar) {
      runLen++;
    } else {
      widths.push(runLen);
      runChar = bits[i];
      runLen = 1;
    }
  }
  widths.push(runLen);

  // Quiet zones: 10 module spaces on each side. We model them as
  // "space" widths at index 0 and at the end. Since widths[] starts on a
  // bar by convention, we fold the leading quiet zone into the caller's
  // SVG layout instead of mutating the widths array.
  const totalBars = widths.reduce((s, w) => s + w, 0);
  return { widths, totalModules: totalBars + 20 };
}
