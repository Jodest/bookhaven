import { describe, expect, it } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AiDraftLine } from '@bookhaven/api-client';
import { pasteLine } from '@/shared/lib/paste-fixtures';
import { defaultDraftCount } from './default-count';
import {
  applyErrorText,
  createDraftErrorText,
  DRAFT_ERROR_CODE_LABELS,
  draftErrorCodeLabel,
} from './draft-error-text';
import { notFoundLines, proposalText } from './not-found-text';

describe('defaultDraftCount', () => {
  it('is the size plus a fifth rounded up, at most 1000', () => {
    expect(defaultDraftCount(100)).toBe(120);
    expect(defaultDraftCount(7)).toBe(9);
    expect(defaultDraftCount(1000)).toBe(1000);
    expect(defaultDraftCount(900)).toBe(1000);
    expect(defaultDraftCount(0)).toBe(1);
  });
});

describe('draft error texts', () => {
  it('labels every error code in Russian', () => {
    expect(draftErrorCodeLabel('model_refusal')).toBe('ИИ отказался составлять список');
    expect(draftErrorCodeLabel('model_truncated')).toBe('Ответ ИИ оборвался — уменьшите количество книг');
    expect(draftErrorCodeLabel('invalid_output')).toBe('ИИ вернул некорректный ответ');
    expect(draftErrorCodeLabel('model_error')).toBe('Ошибка обращения к ИИ');
    expect(draftErrorCodeLabel('enqueue_failed')).toBe('Не удалось поставить в очередь');
    expect(draftErrorCodeLabel('timeout')).toBe('Черновик не был составлен за час');
    expect(Object.keys(DRAFT_ERROR_CODE_LABELS)).toHaveLength(6);
    expect(draftErrorCodeLabel(null)).toBe('Не удалось составить черновик');
  });

  it('maps the create errors', () => {
    expect(createDraftErrorText(new ApiError(409, 'Conflict', 'x'))).toBe('Черновик для этой подборки уже составляется');
    expect(createDraftErrorText(new ApiError(429, 'Too Many', 'x'))).toBe('Лимит черновиков на сегодня исчерпан');
    expect(createDraftErrorText(new ApiError(503, 'Unavailable', 'x'))).toBe('Черновики ИИ выключены');
    expect(createDraftErrorText(new ApiError(502, 'Bad Gateway', 'x'))).toContain('очередь');
    expect(createDraftErrorText(new Error('boom'))).toBe('Не удалось создать черновик.');
  });

  it('maps the apply errors', () => {
    expect(applyErrorText(null)).toBeUndefined();
    expect(applyErrorText(new ApiError(422, 'Unprocessable', 'collection is full'))).toContain('Не хватает мест');
  });
});

describe('not-found text', () => {
  const line = (n: number, status: AiDraftLine['status'], title: string, author: string | null): AiDraftLine => ({
    ...pasteLine(n, status, { title, author, text: title }),
    original_title: null,
    note: '',
  });

  it('writes «Название — Автор» lines for the not_found entries only', () => {
    const lines = [
      line(1, 'found', 'Гиперион', 'Дэн Симмонс'),
      line(2, 'not_found', 'Дюна', 'Фрэнк Герберт'),
      line(3, 'not_found', 'Солярис', null),
    ];
    expect(notFoundLines(lines)).toBe('Дюна — Фрэнк Герберт\nСолярис');
    expect(proposalText(lines[1])).toBe('Дюна — Фрэнк Герберт');
    expect(notFoundLines([lines[0]])).toBe('');
  });
});
