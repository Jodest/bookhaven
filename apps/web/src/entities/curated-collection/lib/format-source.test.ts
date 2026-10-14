import { describe, expect, it } from 'vitest';
import { formatBooksCount, formatSource } from './format-source';

describe('formatSource', () => {
  it('prefixes the source', () => {
    expect(formatSource('по версии BBC Big Read, 2003')).toBe('Источник: по версии BBC Big Read, 2003');
  });

  it('is null without a source', () => {
    expect(formatSource(null)).toBeNull();
    expect(formatSource('  ')).toBeNull();
  });
});

describe('formatBooksCount', () => {
  it.each([
    [1, '1 книга'],
    [3, '3 книги'],
    [11, '11 книг'],
    [100, '100 книг'],
    [101, '101 книга'],
  ])('%i', (n, text) => {
    expect(formatBooksCount(n)).toBe(text);
  });
});
