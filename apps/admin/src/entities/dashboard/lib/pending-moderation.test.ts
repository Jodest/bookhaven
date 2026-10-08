import { describe, expect, it } from 'vitest';
import { pendingModerationTotal } from './pending-moderation';

describe('pendingModerationTotal', () => {
  it('sums pending import review and pending authors, excluding pending_reviews', () => {
    expect(
      pendingModerationTotal({
        books: 100,
        hidden_books: 10,
        authors: 50,
        genres: 5,
        pending_reviews: 99,
        pending_import_review: 4,
        pending_authors: 7,
      })
    ).toBe(11);
  });

  it('is 0 when both queues are empty', () => {
    expect(
      pendingModerationTotal({
        books: 1,
        hidden_books: 0,
        authors: 1,
        genres: 1,
        pending_reviews: 3,
        pending_import_review: 0,
        pending_authors: 0,
      })
    ).toBe(0);
  });
});
