import type { AdminAuthorBulkStatusReport, AdminAuthorStatus } from '@bookhaven/api-client';

/** Confirm text for a single-row status change. */
export function authorStatusConfirmText(
  status: AdminAuthorStatus,
  name: string,
  booksVisible = 0
): string {
  if (status === 'published') {
    return `Опубликовать автора «${name}»?`;
  }
  if (status === 'pending') {
    return `Вернуть автора «${name}» на модерацию?`;
  }
  let text = `Отклонить автора «${name}»? Харвестер больше не будет создавать его заново.`;
  if (booksVisible > 0) {
    text += ` Связи с книгами сохранятся: имя останется без ссылки на страницах опубликованных книг (${booksVisible}).`;
  }
  return text;
}

/** Confirm text for a bulk status change of `count` selected authors. */
export function bulkAuthorStatusConfirmText(
  status: AdminAuthorStatus,
  count: number,
  booksVisible = 0
): string {
  if (status === 'published') {
    return `Опубликовать выбранных авторов (${count})?`;
  }
  if (status === 'pending') {
    return `Вернуть выбранных авторов на модерацию (${count})?`;
  }
  let text = `Отклонить выбранных авторов (${count})? Харвестер больше не будет создавать их заново.`;
  if (booksVisible > 0) {
    text += ` Связи с книгами сохранятся: имена останутся без ссылки на страницах опубликованных книг (${booksVisible}).`;
  }
  return text;
}

/**
 * Feedback text for a bulk-status report: "Обновлено X из N" plus the first
 * three row errors. names maps author ids to display names (the report's
 * own name field is empty for unknown ids).
 */
export function bulkAuthorStatusSummary(
  report: AdminAuthorBulkStatusReport,
  names: Record<string, string> = {}
): string {
  const summary = `Обновлено ${report.updated} из ${report.processed}.`;
  if (report.errors.length === 0) {
    return summary;
  }
  const sample = report.errors
    .slice(0, 3)
    .map((e) => `«${e.name || names[e.author_id] || e.author_id}»: ${e.message}`)
    .join('; ');
  return `${summary} Ошибок: ${report.errors.length} (${sample}).`;
}
