import { describe, expect, it } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { pasteErrorText } from './paste-error-text';

describe('pasteErrorText', () => {
  it('is undefined without an error', () => {
    expect(pasteErrorText(null)).toBeUndefined();
  });

  it('explains a full collection', () => {
    expect(pasteErrorText(new ApiError(422, 'Unprocessable', 'collection is full'))).toBe(
      'Не хватает мест в подборке (максимум 1000 книг)'
    );
  });

  it('explains too many lines', () => {
    expect(pasteErrorText(new ApiError(422, 'Unprocessable', 'too many lines (max 500)'))).toBe(
      'Слишком много строк — не больше 500'
    );
  });

  it('falls back to the message', () => {
    expect(pasteErrorText(new ApiError(400, 'Bad', 'text: must contain at least one line'))).toBe(
      'text: must contain at least one line'
    );
    expect(pasteErrorText('x')).toBe('Не удалось выполнить запрос');
  });
});
