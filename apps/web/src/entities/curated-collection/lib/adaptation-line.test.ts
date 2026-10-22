import { describe, expect, it } from 'vitest';
import type { CollectionSourceAdaptation } from '@bookhaven/api-client';
import { adaptationLine } from './adaptation-line';

const adaptation = (over: Partial<CollectionSourceAdaptation> = {}): CollectionSourceAdaptation => ({
  id: 'w1',
  title: 'Война и мир',
  year: 1967,
  kind: 'film',
  votes_count: 40,
  share: 0.9,
  ...over,
});

describe('adaptationLine', () => {
  it('book_beats_adaptation', () => {
    expect(adaptationLine('book_beats_adaptation', adaptation())).toBe(
      'Книга лучше экранизации «Война и мир» (1967): 90% · 40 голосов'
    );
  });

  it('adaptation_beats_book without a year', () => {
    expect(
      adaptationLine('adaptation_beats_book', adaptation({ title: 'Сталкер', year: null, votes_count: 21, share: 0.62 }))
    ).toBe('Экранизация «Сталкер» лучше книги: 62% · 21 голос');
  });

  it('plural forms of the votes', () => {
    const line = (n: number) => adaptationLine('book_beats_adaptation', adaptation({ votes_count: n }));
    expect(line(1)).toMatch(/ 1 голос$/);
    expect(line(2)).toMatch(/ 2 голоса$/);
    expect(line(5)).toMatch(/ 5 голосов$/);
    expect(line(21)).toMatch(/ 21 голос$/);
  });

  it('rounds the share to a whole percent', () => {
    expect(adaptationLine('book_beats_adaptation', adaptation({ share: 0.884 }))).toContain(': 88% ·');
  });

  it('is null without an adaptation or for other metrics', () => {
    expect(adaptationLine('book_beats_adaptation', null)).toBeNull();
    expect(adaptationLine('weighted_rating', adaptation())).toBeNull();
  });
});
