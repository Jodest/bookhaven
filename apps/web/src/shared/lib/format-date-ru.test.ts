import { describe, expect, it } from 'vitest';
import { formatDayMonth, formatRatingSummary, formatYears, pluralRu } from './format-date-ru';

describe('formatDayMonth', () => {
  it('formats civil dates in Russian independent of the host timezone', () => {
    expect(formatDayMonth('2026-10-04')).toBe('4 октября');
    expect(formatDayMonth('2027-01-01')).toBe('1 января');
    expect(formatDayMonth('2028-02-29')).toBe('29 февраля');
  });
});

describe('pluralRu', () => {
  const forms: [string, string, string] = ['год', 'года', 'лет'];
  it.each([
    [1, 'год'],
    [2, 'года'],
    [5, 'лет'],
    [11, 'лет'],
    [21, 'год'],
    [22, 'года'],
    [25, 'лет'],
    [112, 'лет'],
  ])('%i -> %s', (n, expected) => {
    expect(pluralRu(n, forms)).toBe(expected);
  });

  it('formatYears', () => {
    expect(formatYears(212)).toBe('212 лет');
    expect(formatYears(1)).toBe('1 год');
    expect(formatYears(2)).toBe('2 года');
  });
});

describe('formatRatingSummary', () => {
  it('uses a comma decimal and plural forms', () => {
    expect(formatRatingSummary(4.6, 12)).toBe('4,6 · 12 оценок');
    expect(formatRatingSummary(5, 1)).toBe('5,0 · 1 оценка');
    expect(formatRatingSummary(4.25, 3)).toMatch(/^4,[23] · 3 оценки$/);
  });

  it('is null without ratings', () => {
    expect(formatRatingSummary(0, 0)).toBeNull();
  });
});
