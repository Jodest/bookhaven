import { ApiError } from '@bookhaven/api-client';
import type { AdminAdaptationSyncItem } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { bulkSyncSummary, hasWikidataQid, syncErrorMessage, syncItemText, syncReportHasErrors } from './sync-text';

function item(overrides: Partial<AdminAdaptationSyncItem> = {}): AdminAdaptationSyncItem {
  return {
    book_id: 'b1',
    title: 'Мастер и Маргарита',
    outcome: 'added',
    found: 3,
    added: 3,
    posters: 0,
    notes: [],
    message: '',
    ...overrides,
  };
}

describe('hasWikidataQid', () => {
  it('needs both the wikidata source and an id', () => {
    expect(hasWikidataQid({ external_source: 'wikidata', external_id: 'Q1' })).toBe(true);
    expect(hasWikidataQid({ external_source: 'wikidata', external_id: null })).toBe(false);
    expect(hasWikidataQid({ external_source: 'openlibrary', external_id: 'OL1' })).toBe(false);
    expect(hasWikidataQid({ external_source: null })).toBe(false);
  });
});

describe('syncItemText', () => {
  it('describes every outcome', () => {
    expect(syncItemText(item())).toBe('Книга «Мастер и Маргарита»: добавлено экранизаций — 3.');
    expect(syncItemText(item({ outcome: 'unchanged', added: 0 }))).toBe(
      'Книга «Мастер и Маргарита»: новых экранизаций нет.'
    );
    expect(syncItemText(item({ outcome: 'skipped', notes: ['no_qid'] }))).toBe(
      'Книга «Мастер и Маргарита»: пропущена — нет QID Wikidata.'
    );
    expect(syncItemText(item({ outcome: 'error', notes: ['wikidata_unavailable'] }))).toBe(
      'Книга «Мастер и Маргарита»: не удалось найти экранизации — Wikidata недоступна.'
    );
    expect(syncItemText(item({ outcome: 'error', title: '', message: 'book not found' }), 'Гомер')).toBe(
      'Книга «Гомер»: не удалось найти экранизации — book not found.'
    );
  });
});

describe('bulkSyncSummary', () => {
  it('summarizes counts, QID-less rows and at most three details', () => {
    const report = {
      processed: 6,
      updated: 1,
      added: 3,
      posters: 0,
      items: [
        item(),
        item({ book_id: 'b2', title: 'А', outcome: 'skipped', notes: ['no_qid'] }),
        item({ book_id: 'b3', title: 'Б', outcome: 'unchanged', added: 0 }),
        item({ book_id: 'b4', title: 'В', outcome: 'error', notes: ['wikidata_unavailable'] }),
        item({ book_id: 'b5', title: 'Г', outcome: 'error', message: 'x' }),
        item({ book_id: 'b6', title: 'Д', outcome: 'error', message: 'y' }),
      ],
    };
    expect(bulkSyncSummary(report, 2)).toBe(
      'Обновлено книг: 1 из 6, добавлено экранизаций: 3. Без QID пропущено: 2. ' +
        'Книга «А»: пропущена — нет QID Wikidata. ' +
        'Книга «В»: не удалось найти экранизации — Wikidata недоступна. ' +
        'Книга «Г»: не удалось найти экранизации — x.'
    );
    expect(syncReportHasErrors(report)).toBe(true);
  });

  it('is just the counts when everything went fine', () => {
    expect(bulkSyncSummary({ processed: 1, updated: 1, added: 2, posters: 0, items: [item({ added: 2 })] })).toBe(
      'Обновлено книг: 1 из 1, добавлено экранизаций: 2.'
    );
  });
});

describe('syncErrorMessage', () => {
  it('maps 502 / 503 and falls back', () => {
    expect(syncErrorMessage(new ApiError(502, 'Bad Gateway', 'x'))).toBe('Wikidata недоступна, попробуйте позже.');
    expect(syncErrorMessage(new ApiError(503, 'Unavailable', 'x'))).toBe('Поиск экранизаций не настроен.');
    expect(syncErrorMessage(new ApiError(400, 'Bad Request', 'x'))).toBe('Не удалось найти экранизации.');
    expect(syncErrorMessage(new Error('boom'))).toBe('Не удалось найти экранизации.');
  });
});

describe('poster counts in the banners', () => {
  it('appends the poster count to a single result when above 0', () => {
    expect(syncItemText(item({ posters: 2 }))).toBe(
      'Книга «Мастер и Маргарита»: добавлено экранизаций — 3. Добавлено постеров: 2.'
    );
    expect(syncItemText(item({ outcome: 'unchanged', added: 0, posters: 2 }))).toBe(
      'Книга «Мастер и Маргарита»: новых экранизаций нет. Добавлено постеров: 2.'
    );
    expect(syncItemText(item({ outcome: 'unchanged', added: 0, posters: 0 }))).toBe(
      'Книга «Мастер и Маргарита»: новых экранизаций нет.'
    );
  });

  it('appends the top-level poster count to the bulk banner', () => {
    expect(
      bulkSyncSummary({ processed: 3, updated: 1, added: 2, posters: 4, items: [item({ added: 2 })] }, 2)
    ).toBe('Обновлено книг: 1 из 3, добавлено экранизаций: 2. Добавлено постеров: 4. Без QID пропущено: 2.');
  });
});
