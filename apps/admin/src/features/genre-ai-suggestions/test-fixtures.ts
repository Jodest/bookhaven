import type { AdminGenreAISuggestion, AdminGenreAISuggestionGenre } from '@bookhaven/api-client';

// Fixtures shared by the feature's tests.

export function suggestionGenre(
  overrides: Partial<AdminGenreAISuggestionGenre> & Pick<AdminGenreAISuggestionGenre, 'id' | 'name'>
): AdminGenreAISuggestionGenre {
  return {
    status: 'published',
    merged: false,
    exists: true,
    wikidata_qid: null,
    books_total: 10,
    sample_books: [],
    ...overrides,
  };
}

export const fantasy = suggestionGenre({ id: 'g-fantasy', name: 'Фэнтези', books_total: 120 });
export const fentezi = suggestionGenre({ id: 'g-fentezi', name: 'Фентези', status: 'pending', books_total: 3 });
export const fantasyEn = suggestionGenre({ id: 'g-en', name: 'Fantasy', status: 'pending', books_total: 2 });

export function mergeSuggestion(overrides: Partial<AdminGenreAISuggestion> = {}): AdminGenreAISuggestion {
  return {
    id: 's-merge',
    kind: 'duplicate',
    status: 'open',
    reason_codes: ['spelling', 'language'],
    reason: 'Это один и тот же жанр.',
    confidence: 0.93,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    created_at: '2026-10-15T10:00:00Z',
    decided_at: null,
    stale: false,
    target: { id: fantasy.id, name: fantasy.name },
    genres: [fantasy, fentezi, fantasyEn],
    applied_target: null,
    ...overrides,
  };
}

export function junkSuggestion(overrides: Partial<AdminGenreAISuggestion> = {}): AdminGenreAISuggestion {
  const genre = suggestionGenre({ id: 'g-junk', name: 'Книги 2020', status: 'pending', books_total: 7 });
  return {
    id: 's-junk',
    kind: 'junk',
    status: 'open',
    reason_codes: ['date'],
    reason: 'Год, а не жанр.',
    confidence: 0.8,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    created_at: '2026-10-15T10:00:00Z',
    decided_at: null,
    stale: false,
    target: null,
    genres: [genre],
    applied_target: null,
    ...overrides,
  };
}
