import type { ScreenWorkPosterSource } from '@bookhaven/api-client';

/** Largest accepted poster (matches the server limit). */
export const MAX_POSTER_BYTES = 5 * 1024 * 1024;

export const POSTER_ACCEPT = 'image/jpeg,image/png,image/webp';

const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp'];

/** Client-side check before sending; null when the file is acceptable. */
export function posterFileError(file: Pick<File, 'type' | 'size'>): string | null {
  if (!ALLOWED_TYPES.includes(file.type)) return 'Нужен файл JPEG, PNG или WebP.';
  if (file.size > MAX_POSTER_BYTES) return 'Файл больше 5 МБ.';
  return null;
}

export const POSTER_SOURCE_LABELS: Record<ScreenWorkPosterSource, string> = {
  manual: 'Загружен вручную',
  commons: 'Wikimedia Commons',
  tmdb: 'TMDB',
};
