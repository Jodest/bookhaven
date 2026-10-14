import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { bookErrorText } from './book-error-text';

describe('bookErrorText', () => {
  it('maps 409 to the duplicate message', () => {
    expect(bookErrorText(new ApiError(409, 'Conflict', 'book already in collection'))).toBe(
      'Книга уже в подборке'
    );
  });

  it('maps 422 rejected and full', () => {
    expect(bookErrorText(new ApiError(422, 'Unprocessable', 'book is rejected'))).toBe(
      'Отклонённую книгу нельзя добавить'
    );
    expect(bookErrorText(new ApiError(422, 'Unprocessable', 'collection is full'))).toBe(
      'В подборке уже 1000 книг'
    );
  });

  it('passes other errors through', () => {
    expect(bookErrorText(new Error('boom'))).toBe('boom');
    expect(bookErrorText(undefined)).toBeUndefined();
  });
});
