import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthorEnrichmentItem } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { bulkEnrichSummary, enrichErrorMessage, enrichItemText, enrichReportHasErrors } from './enrich-text';

function item(overrides: Partial<AdminAuthorEnrichmentItem> = {}): AdminAuthorEnrichmentItem {
  return {
    author_id: 'a1',
    name: 'Лев Толстой',
    outcome: 'enriched',
    filled: [],
    notes: [],
    message: '',
    ...overrides,
  };
}

describe('enrichItemText', () => {
  it('lists filled fields', () => {
    expect(enrichItemText(item({ filled: ['birth_date', 'death_date', 'photo'] }))).toBe(
      'Автор «Лев Толстой»: добавлены дата рождения, дата смерти, фото.'
    );
  });

  it('reports nothing to add with notes', () => {
    expect(enrichItemText(item({ outcome: 'unchanged', notes: ['photo_license_not_free'] }))).toBe(
      'Автор «Лев Толстой»: нечего добавить — поля уже заполнены или в Wikidata нет подходящих данных. Примечания: фото под несвободной лицензией — пропущено.'
    );
  });

  it('appends notes to filled fields', () => {
    expect(enrichItemText(item({ filled: ['photo'], notes: ['birth_date_unusable'] }))).toBe(
      'Автор «Лев Толстой»: добавлены фото. Примечания: дата рождения в Wikidata неточная или противоречивая — пропущена.'
    );
  });

  it('describes skipped and error items', () => {
    expect(enrichItemText(item({ outcome: 'skipped', notes: ['no_qid'] }))).toBe(
      'Автор «Лев Толстой»: пропущен — у автора нет QID.'
    );
    expect(enrichItemText(item({ outcome: 'skipped', notes: ['entity_not_found'] }))).toBe(
      'Автор «Лев Толстой»: пропущен — элемент Wikidata не найден.'
    );
    expect(enrichItemText(item({ outcome: 'error', name: '', message: 'author not found' }), 'Гомер')).toBe(
      'Автор «Гомер»: не удалось обогатить — author not found.'
    );
  });
});

describe('bulkEnrichSummary', () => {
  it('summarizes counts, QID-less rows and the first three details', () => {
    const report = {
      processed: 5,
      enriched: 1,
      items: [
        item({ filled: ['photo'] }),
        item({ author_id: 'a2', name: 'А', outcome: 'skipped', notes: ['entity_not_found'] }),
        item({ author_id: 'a3', name: 'Б', outcome: 'unchanged', notes: ['photo_unavailable'] }),
        item({ author_id: 'a4', name: 'В', outcome: 'error', notes: ['wikidata_unavailable'] }),
        item({ author_id: 'a5', name: 'Г', outcome: 'error', message: 'x' }),
      ],
    };
    expect(bulkEnrichSummary(report, 2)).toBe(
      'Обогащено 1 из 5. Без QID пропущено: 2. ' +
        'Автор «А»: пропущен — элемент Wikidata не найден. ' +
        'Автор «Б»: нечего добавить — поля уже заполнены или в Wikidata нет подходящих данных. Примечания: файл фото недоступен на Commons. ' +
        'Автор «В»: не удалось обогатить — Wikidata недоступна.'
    );
    expect(enrichReportHasErrors(report)).toBe(true);
  });

  it('is just the count when everything went fine', () => {
    expect(bulkEnrichSummary({ processed: 1, enriched: 1, items: [item({ filled: ['photo'] })] })).toBe(
      'Обогащено 1 из 1.'
    );
  });
});

describe('enrichErrorMessage', () => {
  it('maps 502 / 503 and falls back', () => {
    expect(enrichErrorMessage(new ApiError(502, 'Bad Gateway', 'x'))).toBe('Wikidata недоступна, попробуйте позже.');
    expect(enrichErrorMessage(new ApiError(503, 'Unavailable', 'x'))).toBe('Обогащение не настроено.');
    expect(enrichErrorMessage(new ApiError(400, 'Bad Request', 'ids must not be empty'))).toBe('ids must not be empty');
    expect(enrichErrorMessage(new Error('boom'))).toBe('Не удалось обогатить авторов.');
  });
});
