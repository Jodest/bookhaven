import { ApiError } from '@bookhaven/api-client';

/** Readable text of a preview or bulk-add error. */
export function pasteErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (err instanceof ApiError && err.status === 422) {
    if (/full/i.test(err.message)) return 'Не хватает мест в подборке (максимум 1000 книг)';
    if (/too many lines/i.test(err.message)) return 'Слишком много строк — не больше 500';
  }
  return err instanceof Error ? err.message : 'Не удалось выполнить запрос';
}
