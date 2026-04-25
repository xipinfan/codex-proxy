import { describe, expect, it } from 'vitest';
import { formatCompactNumber, formatTokenCompact } from '../lib/format';

describe('number formatting', () => {
  it('keeps useful precision for hundred-million scale dashboard totals', () => {
    expect(formatCompactNumber(134_056_317)).toBe('1.34亿');
    expect(formatTokenCompact(134_056_317)).toBe('1.34亿');
  });

  it('uses compact token units without dropping smaller large values', () => {
    expect(formatTokenCompact(23_956_597)).toBe('2395.7万');
    expect(formatTokenCompact(599_969)).toBe('60万');
  });
});
