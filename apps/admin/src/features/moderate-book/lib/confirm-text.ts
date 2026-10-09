import type { AdminBookBulkStatusReport, AdminBookStatus } from '@bookhaven/api-client';

/** Confirm text for a single-row status change. */
export function bookStatusConfirmText(status: AdminBookStatus, title: string): string {
  if (status === 'published') {
    return `Опубликовать книгу «${title}»?`;
  }
  if (status === 'pending') {
    return `Вернуть книгу «${title}» на модерацию?`;
  }
  return `Отклонить книгу «${title}»?`;
}

/** Confirm text for a bulk status change of `count` selected books. */
export function bulkBookStatusConfirmText(status: AdminBookStatus, count: number): string {
  if (status === 'published') {
    return `Опубликовать выбранные книги (${count})?`;
  }
  if (status === 'pending') {
    return `Вернуть выбранные книги на модерацию (${count})?`;
  }
  return `Отклонить выбранные книги (${count})?`;
}

/**
 * Feedback text for a bulk-status report: "Обновлено X из N" plus the first
 * three row errors. names maps book ids to titles (the report's own title
 * field is empty for unknown ids).
 */
export function bulkBookStatusSummary(
  report: AdminBookBulkStatusReport,
  names: Record<string, string> = {}
): string {
  const summary = `Обновлено ${report.updated} из ${report.processed}.`;
  if (report.errors.length === 0) {
    return summary;
  }
  const sample = report.errors
    .slice(0, 3)
    .map((e) => `«${e.title || names[e.book_id] || e.book_id}»: ${e.message}`)
    .join('; ');
  return `${summary} Ошибок: ${report.errors.length} (${sample}).`;
}
