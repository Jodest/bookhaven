import { ApiError } from '@bookhaven/api-client';

// Server errors of the screen work endpoints in Russian
// (openspec screen-works/admin-editing).

function status(err: unknown): number | null {
  return err instanceof ApiError ? err.status : null;
}

function message(err: unknown): string {
  return err instanceof ApiError ? err.message : '';
}

/** Errors of POST …/adaptations and POST …/screen-works. */
export function linkWorkErrorText(err: unknown, fallback: string): string {
  if (status(err) === 409) {
    const msg = message(err);
    if (msg.includes('imdb id')) {
      return 'Фильм с таким IMDb id уже есть в каталоге — выберите его в поиске.';
    }
    if (msg.includes('kinopoisk id')) {
      return 'Фильм с таким Кинопоиск id уже есть в каталоге — выберите его в поиске.';
    }
    if (msg.includes('already linked')) return 'Этот фильм уже привязан.';
  }
  return fallback;
}

/** Errors of PATCH /admin/screen-works/{id}. */
export function saveWorkErrorText(err: unknown): string {
  if (status(err) === 404) return 'Фильм не найден.';
  if (status(err) === 409 && message(err).includes('qid')) return 'Фильм с таким QID уже есть в каталоге.';
  return 'Не удалось сохранить фильм.';
}

/** Errors of DELETE /admin/screen-works/{id}. */
export function deleteWorkErrorText(err: unknown): string {
  if (status(err) === 404) return 'Фильм не найден.';
  return 'Не удалось удалить фильм.';
}
