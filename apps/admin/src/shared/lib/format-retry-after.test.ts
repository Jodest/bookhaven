import { describe, expect, it } from 'vitest';
import { formatRetryAfter } from './format-retry-after';

describe('formatRetryAfter', () => {
  it.each([
    [1, '1 сек'],
    [59, '59 сек'],
    [60, '1 мин'],
    [61, '2 мин'],
    [3599, '60 мин'],
    [3600, '1 ч'],
    [86400, '24 ч'],
  ])('%i -> %s', (seconds, expected) => {
    expect(formatRetryAfter(seconds)).toBe(expected);
  });

  it('clamps non-positive values to 1 second', () => {
    expect(formatRetryAfter(0)).toBe('1 сек');
  });
});
