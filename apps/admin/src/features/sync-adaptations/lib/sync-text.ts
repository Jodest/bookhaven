import { ApiError } from '@bookhaven/api-client';
import type { AdminAdaptationSyncItem, AdminAdaptationSyncReport } from '@bookhaven/api-client';

interface BookWithSource {
  external_source?: string | null;
  external_id?: string | null;
}

/** Whether the book has a Wikidata QID, so the sync action applies to it. */
export function hasWikidataQid(book: BookWithSource): boolean {
  return book.external_source === 'wikidata' && !!book.external_id;
}

function notesText(notes: string[]): string {
  const parts: string[] = [];
  if (notes.includes('no_qid')) parts.push('нет QID Wikidata');
  if (notes.includes('wikidata_unavailable')) parts.push('Wikidata недоступна');
  return parts.join(', ');
}

/** Result line for one book (single-row action and bulk details). */
export function syncItemText(item: AdminAdaptationSyncItem, fallbackTitle = ''): string {
  const title = item.title || fallbackTitle || item.book_id;

  if (item.outcome === 'error') {
    const reason = notesText(item.notes) || item.message || 'неизвестная ошибка';
    return `Книга «${title}»: не удалось найти экранизации — ${reason}.`;
  }
  if (item.outcome === 'skipped') {
    return `Книга «${title}»: пропущена — ${notesText(item.notes) || 'нет данных'}.`;
  }
  if (item.outcome === 'added') {
    return `Книга «${title}»: добавлено экранизаций — ${item.added}.`;
  }
  return `Книга «${title}»: новых экранизаций нет.`;
}

/**
 * Bulk banner: «Обновлено книг: n из m, добавлено экранизаций: k.», the
 * client-side count of selected rows without a QID (not sent), and the
 * first three per-book errors/skips.
 */
export function bulkSyncSummary(
  report: AdminAdaptationSyncReport,
  skippedWithoutQid = 0,
  titles: Record<string, string> = {}
): string {
  let text = `Обновлено книг: ${report.updated} из ${report.processed}, добавлено экранизаций: ${report.added}.`;
  if (skippedWithoutQid > 0) {
    text += ` Без QID пропущено: ${skippedWithoutQid}.`;
  }
  const details = report.items
    .filter((i) => i.outcome === 'error' || i.outcome === 'skipped')
    .slice(0, 3)
    .map((i) => syncItemText(i, titles[i.book_id]));
  if (details.length > 0) {
    text += ` ${details.join(' ')}`;
  }
  return text;
}

/** Banner text for a failed sync call. */
export function syncErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 502) return 'Wikidata недоступна, попробуйте позже.';
    if (err.status === 503) return 'Поиск экранизаций не настроен.';
  }
  return 'Не удалось найти экранизации.';
}

/** Whether the banner for a report should be styled as an error. */
export function syncReportHasErrors(report: AdminAdaptationSyncReport): boolean {
  return report.items.some((i) => i.outcome === 'error');
}
