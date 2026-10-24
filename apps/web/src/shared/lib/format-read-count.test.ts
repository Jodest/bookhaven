import { describe, expect, it } from 'vitest';
import { formatReadCount, formatReadCounter } from './format-read-count';

describe('formatReadCount', () => {
  it.each([
    [1, 'Прочитано 1 раз'],
    [2, 'Прочитано 2 раза'],
    [5, 'Прочитано 5 раз'],
    [11, 'Прочитано 11 раз'],
    [12, 'Прочитано 12 раз'],
    [21, 'Прочитано 21 раз'],
    [22, 'Прочитано 22 раза'],
  ])('%i', (n, expected) => {
    expect(formatReadCount(n)).toBe(expected);
  });
});

describe('formatReadCounter', () => {
  it('adds the nominative lowercase month and year of the last read', () => {
    expect(formatReadCounter(2, new Date(2025, 4, 15, 12).toISOString())).toBe(
      'Прочитано 2 раза · последний раз — май 2025'
    );
    expect(formatReadCounter(1, new Date(2024, 2, 10, 12).toISOString())).toBe(
      'Прочитано 1 раз · последний раз — март 2024'
    );
  });

  it('omits the date when there is none', () => {
    expect(formatReadCounter(3, null)).toBe('Прочитано 3 раза');
  });
});
