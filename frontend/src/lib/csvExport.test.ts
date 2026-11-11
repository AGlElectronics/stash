import { describe, expect, it } from 'vitest';
import { csvEscape, csvFormulaSafe, rowsToCsv } from './csvExport';

describe('csvFormulaSafe', () => {
  it.each(['=HYPERLINK("http://x")', '+1+1', '-2+3', '@SUM(A1)', '\tcmd', '\rcmd'])(
    'prefixes formula-like text %s',
    (v) => {
      expect(csvFormulaSafe(v)).toBe(`'${v}`);
    }
  );

  it.each(['-5', '+3.5', '-1e3', 'plain', '', 'a=b'])('leaves %s alone', (v) => {
    expect(csvFormulaSafe(v)).toBe(v);
  });
});

describe('csvEscape', () => {
  it('quotes and escapes after making formulas safe', () => {
    expect(csvEscape('=1,2')).toBe(`"'=1,2"`);
    expect(csvEscape('say "hi"')).toBe(`"say ""hi"""`);
  });

  it('builds CRLF rows', () => {
    expect(rowsToCsv([['a', '=b'], ['1', '2']])).toBe(`a,'=b\r\n1,2`);
  });
});
