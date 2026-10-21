import type { AdaptationVerdict } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { verdictLabel } from './verdict-label';

function verdict(over: Partial<AdaptationVerdict>): AdaptationVerdict {
  return {
    screen_work_id: 'w1',
    votes_count: 0,
    book_better: null,
    adaptation_better: null,
    same: null,
    my_vote: null,
    ...over,
  };
}

describe('verdictLabel', () => {
  it('shows the leader, its share and the vote count', () => {
    expect(
      verdictLabel(verdict({ votes_count: 23, book_better: 18, adaptation_better: 3, same: 2 }), 5),
    ).toBe('78% читателей: книга лучше · 23 голоса');
    expect(
      verdictLabel(verdict({ votes_count: 6, book_better: 1, adaptation_better: 4, same: 1 }), 5),
    ).toBe('67% читателей: экранизация лучше · 6 голосов');
    expect(
      verdictLabel(verdict({ votes_count: 5, book_better: 0, adaptation_better: 1, same: 4 }), 5),
    ).toBe('80% читателей: одинаково · 5 голосов');
  });

  it('reports a tie for the lead', () => {
    expect(
      verdictLabel(verdict({ votes_count: 6, book_better: 3, adaptation_better: 3, same: 0 }), 5),
    ).toBe('Мнения разделились · 6 голосов');
  });

  it('asks for more votes below the minimum or without counts', () => {
    expect(verdictLabel(verdict({ votes_count: 3 }), 5)).toBe('Пока мало голосов');
    expect(verdictLabel(verdict({ votes_count: 0 }), 5)).toBe('Пока мало голосов');
    expect(verdictLabel(verdict({ votes_count: 9 }), 5)).toBe('Пока мало голосов');
  });

  it('uses the right plural form', () => {
    const one = verdict({ votes_count: 1, book_better: 1, adaptation_better: 0, same: 0 });
    expect(verdictLabel(one, 1)).toBe('100% читателей: книга лучше · 1 голос');
    const two = verdict({ votes_count: 2, book_better: 2, adaptation_better: 0, same: 0 });
    expect(verdictLabel(two, 1)).toBe('100% читателей: книга лучше · 2 голоса');
    const five = verdict({ votes_count: 5, book_better: 5, adaptation_better: 0, same: 0 });
    expect(verdictLabel(five, 5)).toBe('100% читателей: книга лучше · 5 голосов');
  });
});
