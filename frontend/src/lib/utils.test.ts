import { describe, expect, it } from 'vitest';
import { cn, formatDate, formatDateTime, formatUptime } from './utils';

describe('cn', () => {
  it('merges tailwind classes', () => {
    expect(cn('p-2', 'p-4')).toBe('p-4');
  });

  it('keeps conditional classes', () => {
    const hide = false;
    expect(cn('text-sm', hide && 'hidden', 'font-bold')).toBe('text-sm font-bold');
  });
});

describe('formatDate / formatDateTime', () => {
  it('returns - for empty input', () => {
    expect(formatDate(null)).toBe('-');
    expect(formatDateTime(undefined)).toBe('-');
  });

  it('returns - for invalid date', () => {
    expect(formatDate('not-a-date')).toBe('-');
  });

  it('formats valid dates', () => {
    expect(formatDate(new Date('2026-05-04T00:00:00Z'))).not.toBe('-');
  });
});

describe('formatUptime', () => {
  it('formats ~1171 minutes as hours and minutes', () => {
    expect(formatUptime(1171 * 60)).toBe('19h 31m');
  });

  it('handles days', () => {
    expect(formatUptime(90061)).toBe('1d 1h');
    expect(formatUptime(86400)).toBe('1d');
  });

  it('handles sub-hour', () => {
    expect(formatUptime(90)).toBe('1m');
    expect(formatUptime(45)).toBe('45s');
  });

  it('clamps negative to zero', () => {
    expect(formatUptime(-1)).toBe('0s');
  });
});
