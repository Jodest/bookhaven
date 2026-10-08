import type { AdminGenreBulkStatusReport, AdminGenreStatus } from '@bookhaven/api-client';

/** Confirm text for a single-row genre status change. */
export function genreStatusConfirmText(
  status: AdminGenreStatus,
  name: string,
  booksVisible = 0
): string {
  if (status === 'published') {
    return `Опубликовать жанр «${name}»?`;
  }
  if (status === 'pending') {
    return `Вернуть жанр «${name}» на модерацию?`;
  }
  let text = `Отклонить жанр «${name}»? Харвестер больше не будет создавать его заново.`;
  if (booksVisible > 0) {
    text += ` Жанр исчезнет со страниц опубликованных книг (${booksVisible}); связи с книгами сохранятся.`;
  }
  return text;
}

/** Confirm text for a bulk status change of `count` selected genres. */
export function bulkGenreStatusConfirmText(
  status: AdminGenreStatus,
  count: number,
  booksVisible = 0
): string {
  if (status === 'published') {
    return `Опубликовать выбранные жанры (${count})?`;
  }
  if (status === 'pending') {
    return `Вернуть выбранные жанры на модерацию (${count})?`;
  }
  let text = `Отклонить выбранные жанры (${count})? Харвестер больше не будет создавать их заново.`;
  if (booksVisible > 0) {
    text += ` Жанры исчезнут со страниц опубликованных книг (${booksVisible}); связи с книгами сохранятся.`;
  }
  return text;
}

/**
 * Feedback text for a bulk-status report: "Обновлено X из N" plus the first
 * three row errors. names maps genre ids to display names (the report's own
 * name field is empty for unknown ids).
 */
export function bulkGenreStatusSummary(
  report: AdminGenreBulkStatusReport,
  names: Record<string, string> = {}
): string {
  const summary = `Обновлено ${report.updated} из ${report.processed}.`;
  if (report.errors.length === 0) {
    return summary;
  }
  const sample = report.errors
    .slice(0, 3)
    .map((e) => `«${e.name || names[e.genre_id] || e.genre_id}»: ${e.message}`)
    .join('; ');
  return `${summary} Ошибок: ${report.errors.length} (${sample}).`;
}
