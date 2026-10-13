import { describe, expect, it } from 'vitest';
import { AdminAuthor, AdminAuthorImportResult, ApiError } from '@bookhaven/api-client';
import type { HarvestStep } from '../model/use-import-wikidata-author';
import { harvestConfirmText, importErrorMessage, importResultFeedback } from './result-text';

function author(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Лев Толстой',
    slug: 'lev-tolstoy',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'pending',
    external_source: 'wikidata',
    external_id: 'Q7200',
    photo_attribution: null,
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function result(overrides: Partial<AdminAuthorImportResult> = {}): AdminAuthorImportResult {
  return {
    outcome: 'created',
    match: null,
    qid_backfilled: false,
    namesake_of: null,
    enrichment: null,
    author: author(),
    ...overrides,
  };
}

const notRequested: HarvestStep = { state: 'not_requested' };
const openHref = (status: string) =>
  `/moderation/authors?${new URLSearchParams({ status, search: 'Лев Толстой' })}`;

describe('importResultFeedback', () => {
  it('created', () => {
    const fb = importResultFeedback({ result: result(), withBooks: false, harvest: notRequested });
    expect(fb).toEqual({
      type: 'success',
      message: 'Автор «Лев Толстой» импортирован и ждёт модерации.',
      links: [],
    });
  });

  it('created with an enrichment summary', () => {
    const fb = importResultFeedback({
      result: result({ enrichment: { filled: ['birth_date', 'death_date', 'photo'], notes: [], error: '' } }),
      withBooks: false,
      harvest: notRequested,
    });
    expect(fb.message).toBe(
      'Автор «Лев Толстой» импортирован и ждёт модерации. Из Wikidata добавлены: дата рождения, дата смерти, фото.'
    );

    const withNotes = importResultFeedback({
      result: result({ enrichment: { filled: [], notes: ['photo_attribution_missing'], error: '' } }),
      withBooks: false,
      harvest: notRequested,
    });
    expect(withNotes.message).toBe(
      'Автор «Лев Толстой» импортирован и ждёт модерации. Из Wikidata ничего не добавлено. Примечания: у фото не указан автор — пропущено.'
    );
  });

  it('created with a namesake', () => {
    const fb = importResultFeedback({
      result: result({ namesake_of: author({ id: 'a0', name: 'Лев Толстой (другой)', external_id: 'Q1' }) }),
      withBooks: false,
      harvest: notRequested,
    });
    expect(fb.message).toBe(
      'Автор «Лев Толстой» импортирован и ждёт модерации. В каталоге уже есть тёзка «Лев Толстой (другой)» с другим QID — создан отдельный автор.'
    );
  });

  it('existing by QID shows the status label and an open link', () => {
    const fb = importResultFeedback({
      result: result({ outcome: 'existing', match: 'qid', author: author({ status: 'published' }) }),
      withBooks: false,
      harvest: notRequested,
    });
    expect(fb.type).toBe('success');
    expect(fb.message).toBe('Автор «Лев Толстой» уже есть в каталоге (опубликован).');
    expect(fb.links).toEqual([{ href: openHref('published'), label: 'Открыть' }]);
  });

  it('existing by slug with QID backfill', () => {
    const fb = importResultFeedback({
      result: result({ outcome: 'existing', match: 'slug', qid_backfilled: true, author: author({ status: 'published' }) }),
      withBooks: false,
      harvest: notRequested,
    });
    expect(fb.message).toBe('Найден автор «Лев Толстой» без QID — QID Q7200 привязан к нему.');
    expect(fb.links).toEqual([{ href: openHref('published'), label: 'Открыть' }]);
  });

  it('rejected, with and without books', () => {
    const rejected = result({ outcome: 'rejected', match: 'qid', author: author({ status: 'rejected' }) });
    const plain = importResultFeedback({ result: rejected, withBooks: false, harvest: notRequested });
    expect(plain.type).toBe('error');
    expect(plain.message).toBe('Автор «Лев Толстой» был отклонён ранее и повторно не создаётся.');
    expect(plain.links).toEqual([{ href: openHref('rejected'), label: 'Открыть' }]);

    const books = importResultFeedback({ result: rejected, withBooks: true, harvest: { state: 'skipped' } });
    expect(books.message).toBe(
      'Автор «Лев Толстой» был отклонён ранее и повторно не создаётся. Книги не импортированы.'
    );
  });

  it('harvest started appends the harvest sentence and link', () => {
    const fb = importResultFeedback({ result: result(), withBooks: true, harvest: { state: 'started' } });
    expect(fb.type).toBe('success');
    expect(fb.message).toBe(
      'Автор «Лев Толстой» импортирован и ждёт модерации. Харвест книг запущен — книги появятся скрытыми в очереди на проверку.'
    );
    expect(fb.links).toEqual([{ href: '/import/jobs', label: 'К задачам харвеста' }]);
  });

  it('harvest failed after import', () => {
    const fb = importResultFeedback({
      result: result(),
      withBooks: true,
      harvest: { state: 'failed', error: 'redis down' },
    });
    expect(fb.type).toBe('error');
    expect(fb.message).toBe(
      'Автор импортирован, но харвест не запущен: redis down. Нажмите «Импортировать книги» ещё раз.'
    );
  });
});

describe('importErrorMessage', () => {
  it('maps 422 codes, 502 and falls back to the message', () => {
    expect(importErrorMessage(new ApiError(422, 'Unprocessable', 'x', '', 'no_ru_label'))).toBe(
      'У элемента Wikidata нет русского названия — импорт невозможен. Создайте автора вручную.'
    );
    expect(importErrorMessage(new ApiError(422, 'Unprocessable', 'x', '', 'not_a_person'))).toBe(
      'Элемент Wikidata не является человеком или псевдонимом — импорт отменён.'
    );
    expect(importErrorMessage(new ApiError(502, 'Bad Gateway', 'wikidata is unavailable'))).toBe(
      'Wikidata недоступна, попробуйте позже.'
    );
    expect(importErrorMessage(new ApiError(404, 'Not Found', 'wikidata entity not found'))).toBe(
      'wikidata entity not found'
    );
    expect(importErrorMessage(new Error('boom'))).toBe('Не удалось импортировать автора.');
  });
});

describe('harvestConfirmText', () => {
  it('names the author', () => {
    expect(harvestConfirmText('Лев Толстой')).toBe(
      'Запустить харвест книг автора «Лев Толстой»? Книги появятся скрытыми в очереди на проверку.'
    );
  });
});
