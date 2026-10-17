import { describe, expect, it } from 'vitest';
import { pluralRu } from './plural-ru';

const forms = ['оценка', 'оценки', 'оценок'] as const;

describe('pluralRu', () => {
  it.each([
    [0, 'оценок'],
    [1, 'оценка'],
    [2, 'оценки'],
    [5, 'оценок'],
    [11, 'оценок'],
    [12, 'оценок'],
    [14, 'оценок'],
    [21, 'оценка'],
    [22, 'оценки'],
    [25, 'оценок'],
    [111, 'оценок'],
  ])('%i -> %s', (n, expected) => {
    expect(pluralRu(n, forms)).toBe(expected);
  });
});
