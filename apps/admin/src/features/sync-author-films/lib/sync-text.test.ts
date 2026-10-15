import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthorFilmSyncItem } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { bulkSyncSummary, syncErrorMessage, syncItemText, syncReportHasErrors } from './sync-text';

function item(overrides: Partial<AdminAuthorFilmSyncItem> = {}): AdminAuthorFilmSyncItem {
  return {
    author_id: 'a1',
    name: 'Толкин',
    outcome: 'added',
    found: 3,
    added: 3,
    notes: [],
    message: '',
    ...overrides,
  };
}

describe('syncItemText', () => {
  it('describes every outcome', () => {
    expect(syncItemText(item())).toBe('Автор «Толкин»: добавлено фильмов — 3.');
    expect(syncItemText(item({ outcome: 'unchanged', added: 0 }))).toBe('Автор «Толкин»: новых фильмов нет.');
    expect(syncItemText(item({ outcome: 'skipped', notes: ['no_qid'] }))).toBe(
      'Автор «Толкин»: пропущен — нет QID Wikidata.'
    );
    expect(syncItemText(item({ outcome: 'error', notes: ['wikidata_unavailable'] }))).toBe(
      'Автор «Толкин»: не удалось найти фильмы — Wikidata недоступна.'
    );
    expect(syncItemText(item({ outcome: 'error', name: '', message: 'author not found' }), 'Гомер')).toBe(
      'Автор «Гомер»: не удалось найти фильмы — author not found.'
    );
  });
});

describe('bulkSyncSummary', () => {
  it('summarizes counts, QID-less rows and at most three details', () => {
    const report = {
      processed: 6,
      updated: 1,
      added: 3,
      items: [
        item(),
        item({ author_id: 'a2', name: 'А', outcome: 'skipped', notes: ['no_qid'] }),
        item({ author_id: 'a3', name: 'Б', outcome: 'unchanged', added: 0 }),
        item({ author_id: 'a4', name: 'В', outcome: 'error', notes: ['wikidata_unavailable'] }),
        item({ author_id: 'a5', name: 'Г', outcome: 'error', message: 'x' }),
        item({ author_id: 'a6', name: 'Д', outcome: 'error', message: 'y' }),
      ],
    };
    expect(bulkSyncSummary(report, 2)).toBe(
      'Обновлено авторов: 1 из 6, добавлено фильмов: 3. Без QID пропущено: 2. ' +
        'Автор «А»: пропущен — нет QID Wikidata. ' +
        'Автор «В»: не удалось найти фильмы — Wikidata недоступна. ' +
        'Автор «Г»: не удалось найти фильмы — x.'
    );
    expect(syncReportHasErrors(report)).toBe(true);
  });

  it('is just the counts when everything went fine', () => {
    expect(bulkSyncSummary({ processed: 1, updated: 1, added: 2, items: [item({ added: 2 })] })).toBe(
      'Обновлено авторов: 1 из 1, добавлено фильмов: 2.'
    );
  });
});

describe('syncErrorMessage', () => {
  it('maps 502 / 503 and falls back', () => {
    expect(syncErrorMessage(new ApiError(502, 'Bad Gateway', 'x'))).toBe('Wikidata недоступна, попробуйте позже.');
    expect(syncErrorMessage(new ApiError(503, 'Unavailable', 'x'))).toBe('Поиск фильмов об авторах не настроен.');
    expect(syncErrorMessage(new ApiError(400, 'Bad Request', 'x'))).toBe('Не удалось найти фильмы об авторе.');
    expect(syncErrorMessage(new Error('boom'))).toBe('Не удалось найти фильмы об авторе.');
  });
});
