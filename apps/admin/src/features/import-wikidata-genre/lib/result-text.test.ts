import { describe, expect, it } from 'vitest';
import { AdminGenre, AdminGenreImportResult, ApiError } from '@bookhaven/api-client';
import { genreImportErrorMessage, genreImportResultFeedback } from './result-text';

function genre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return { id: 'g1', name: 'фэнтези', slug: 'fentezi', status: 'pending', wikidata_qid: 'Q132311', ...overrides };
}

function result(overrides: Partial<AdminGenreImportResult> = {}): AdminGenreImportResult {
  return {
    outcome: 'created',
    match: null,
    qid_backfilled: false,
    qid_mismatch: null,
    via_alias: null,
    genre: genre(),
    ...overrides,
  };
}

const openHref = (status: string, name = 'фэнтези') =>
  `/moderation/genres?${new URLSearchParams({ status, search: name })}`;
const MODERATION = { href: '/moderation/genres', label: 'К модерации жанров' };

describe('genreImportResultFeedback', () => {
  it('created', () => {
    expect(genreImportResultFeedback(result())).toEqual({
      type: 'success',
      message: 'Жанр «фэнтези» импортирован и ждёт модерации.',
      links: [{ href: openHref('pending'), label: 'Открыть' }, MODERATION],
    });
  });

  it('existing by QID, direct', () => {
    const fb = genreImportResultFeedback(
      result({ outcome: 'existing', match: 'qid', genre: genre({ status: 'published' }) })
    );
    expect(fb).toEqual({
      type: 'success',
      message: 'Жанр «фэнтези» уже есть в каталоге (опубликован).',
      links: [{ href: openHref('published'), label: 'Открыть' }],
    });
  });

  it('existing via a merged alias (qid or slug)', () => {
    for (const match of ['qid', 'slug'] as const) {
      const fb = genreImportResultFeedback(
        result({
          outcome: 'existing',
          match,
          via_alias: genre({ id: 'g2', name: 'фентези', slug: 'fentezi-2', status: 'rejected', wikidata_qid: 'Q5' }),
          genre: genre({ status: 'published' }),
        })
      );
      expect(fb).toEqual({
        type: 'success',
        message: 'Этот жанр был слит в «фэнтези» (опубликован) — новый жанр не создан.',
        links: [{ href: openHref('published'), label: 'Открыть' }],
      });
    }
  });

  it('existing by slug with the QID backfilled', () => {
    const fb = genreImportResultFeedback(
      result({ outcome: 'existing', match: 'slug', qid_backfilled: true, genre: genre({ status: 'published' }) })
    );
    expect(fb).toEqual({
      type: 'success',
      message: 'Найден жанр «фэнтези» без QID — QID Q132311 привязан к нему.',
      links: [{ href: openHref('published'), label: 'Открыть' }],
    });
  });

  it('existing by slug with a different QID', () => {
    const fb = genreImportResultFeedback(
      result({
        outcome: 'existing',
        match: 'slug',
        qid_mismatch: 'Q99',
        genre: genre({ status: 'published', wikidata_qid: 'Q99' }),
      })
    );
    expect(fb).toEqual({
      type: 'success',
      message:
        'Жанр «фэнтези» уже есть в каталоге с другим QID (Q99) — новый жанр не создан. ' +
        'Если это дубликат или другое понятие, разберитесь в модерации.',
      links: [{ href: openHref('published'), label: 'Открыть' }, MODERATION],
    });
  });

  it('existing by slug, same QID', () => {
    const fb = genreImportResultFeedback(result({ outcome: 'existing', match: 'slug' }));
    expect(fb.type).toBe('success');
    expect(fb.message).toBe('Жанр «фэнтези» уже есть в каталоге (на модерации).');
  });

  it('rejected, with and without a backfill', () => {
    const rejected = genre({ status: 'rejected' });
    expect(genreImportResultFeedback(result({ outcome: 'rejected', match: 'qid', genre: rejected }))).toEqual({
      type: 'error',
      message: 'Жанр «фэнтези» был отклонён ранее и повторно не создаётся.',
      links: [{ href: openHref('rejected'), label: 'Открыть' }],
    });
    expect(
      genreImportResultFeedback(result({ outcome: 'rejected', match: 'slug', qid_backfilled: true, genre: rejected }))
        .message
    ).toBe('Жанр «фэнтези» был отклонён ранее и повторно не создаётся. QID привязан к нему.');
  });
});

describe('genreImportErrorMessage', () => {
  it('maps 422 codes', () => {
    expect(genreImportErrorMessage(new ApiError(422, 'Unprocessable Entity', 'x', '', 'no_ru_label'))).toBe(
      'У элемента Wikidata нет русского названия — импорт невозможен. Создайте жанр вручную в каталоге.'
    );
    expect(genreImportErrorMessage(new ApiError(422, 'Unprocessable Entity', 'x', '', 'not_a_literary_genre'))).toBe(
      'Элемент Wikidata не является литературным жанром — импорт отменён.'
    );
    expect(genreImportErrorMessage(new ApiError(422, 'Unprocessable Entity', 'something else'))).toBe(
      'something else'
    );
  });

  it('maps 502 / 503 and falls back to the message', () => {
    expect(genreImportErrorMessage(new ApiError(502, 'Bad Gateway', 'x'))).toBe('Wikidata недоступна, попробуйте позже.');
    expect(genreImportErrorMessage(new ApiError(503, 'Service Unavailable', 'x'))).toBe(
      'Импорт жанров не настроен на сервере.'
    );
    expect(genreImportErrorMessage(new ApiError(409, 'Conflict', 'genre was created concurrently; retry'))).toBe(
      'genre was created concurrently; retry'
    );
    expect(genreImportErrorMessage(new Error('boom'))).toBe('Не удалось импортировать жанр.');
  });
});
