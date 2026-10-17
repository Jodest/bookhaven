import { describe, expect, it } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import {
  analyzeErrorMessage,
  applyErrorMessage,
  bulkApplyMessage,
  bulkDismissConfirmText,
  bulkDismissMessage,
  dismissErrorMessage,
  junkConfirmText,
  junkSuccessMessage,
  mergeSuccessMessage,
} from './messages';

const genre = { id: 'g', name: 'Фэнтези', slug: 'f', status: 'published' as const, wikidata_qid: null };

describe('suggestion messages', () => {
  it('reports a merge, with the switched collections only when there are some', () => {
    expect(
      mergeSuccessMessage({ genre, merge_id: 'm', sources_merged: 2, books_reassigned: 5, collections_switched: 0 }, 'Фэнтези')
    ).toBe('Жанры слиты в «Фэнтези». Перенесено книг: 5.');
    expect(
      mergeSuccessMessage({ genre, merge_id: 'm', sources_merged: 2, books_reassigned: 5, collections_switched: 2 }, 'Фэнтези')
    ).toBe('Жанры слиты в «Фэнтези». Перенесено книг: 5. Переключено подборок: 2.');
  });

  it('reports junk, bulk apply and dismissals', () => {
    expect(junkSuccessMessage('Книги 2020')).toBe('Жанр «Книги 2020» отклонён.');
    expect(junkConfirmText('Книги 2020', 7)).toBe(
      'Отклонить жанр «Книги 2020»? Связи с книгами (7) сохранятся, жанр исчезнет со страниц книг.'
    );
    expect(
      bulkApplyMessage({ processed: 4, applied: 2, stale: 1, errors: [{ suggestion_id: 'x', message: 'm' }] })
    ).toBe('Применено: 2. Устарело: 1. Ошибок: 1.');
    expect(bulkDismissConfirmText(3)).toBe(
      'Отклонить выбранные предложения (3)? ИИ больше не предложит их для тех же жанров.'
    );
    expect(bulkDismissMessage({ processed: 3, dismissed: 2, errors: [] })).toBe('Отклонено предложений: 2.');
  });

  it('maps apply errors by status', () => {
    expect(applyErrorMessage(new ApiError(409, 'Conflict', 'suggestion is stale'))).toBe(
      'Предложение устарело: жанры изменились после анализа.'
    );
    expect(applyErrorMessage(new ApiError(409, 'Conflict', 'suggestion is not open'))).toBe(
      'Предложение уже обработано. Обновите список.'
    );
    expect(applyErrorMessage(new ApiError(404, 'Not Found', 'genre not found'))).toMatch(/не найден.*уже слили/);
    expect(applyErrorMessage(new ApiError(404, 'Not Found', 'suggestion not found'))).toBe(
      'Предложение не найдено. Обновите список.'
    );
    expect(applyErrorMessage(new ApiError(400, 'Bad Request', 'x'))).toBe(
      'Не удалось применить предложение. Попробуйте позже.'
    );
    expect(applyErrorMessage(new Error('network'))).toBe('Не удалось применить предложение. Попробуйте позже.');
  });

  it('maps dismiss and analyze errors', () => {
    expect(dismissErrorMessage(new ApiError(409, 'Conflict', 'suggestion is not open'))).toBe(
      'Предложение уже обработано. Обновите список.'
    );
    expect(dismissErrorMessage(new Error('x'))).toBe('Не удалось отклонить предложение. Попробуйте позже.');
    expect(analyzeErrorMessage(new ApiError(409, 'Conflict', 'AI genre analysis already running'))).toBe(
      'Анализ уже идёт.'
    );
    expect(analyzeErrorMessage(new ApiError(503, 'Unavailable', 'AI genre analysis is disabled'))).toBe(
      'ИИ-анализ жанров выключен.'
    );
    expect(analyzeErrorMessage(new ApiError(502, 'Bad Gateway', 'x'))).toBe(
      'Не удалось запустить анализ. Попробуйте позже.'
    );
  });
});
