import { ApiError } from '@bookhaven/api-client';

/** Text of an add/remove/move book error. */
export function bookErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (err instanceof ApiError) {
    if (err.status === 409) return 'Книга уже в подборке';
    if (err.status === 422) {
      return /full/i.test(err.message) ? 'В подборке уже 1000 книг' : 'Отклонённую книгу нельзя добавить';
    }
  }
  return err instanceof Error ? err.message : undefined;
}
