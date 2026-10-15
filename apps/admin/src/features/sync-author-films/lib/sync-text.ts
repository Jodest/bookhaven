import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthorFilmSyncItem, AdminAuthorFilmSyncReport } from '@bookhaven/api-client';

function notesText(notes: string[]): string {
  const parts: string[] = [];
  if (notes.includes('no_qid')) parts.push('нет QID Wikidata');
  if (notes.includes('wikidata_unavailable')) parts.push('Wikidata недоступна');
  return parts.join(', ');
}

/** Result line for one author (single-row action and bulk details). */
export function syncItemText(item: AdminAuthorFilmSyncItem, fallbackName = ''): string {
  const name = item.name || fallbackName || item.author_id;

  if (item.outcome === 'error') {
    const reason = notesText(item.notes) || item.message || 'неизвестная ошибка';
    return `Автор «${name}»: не удалось найти фильмы — ${reason}.`;
  }
  if (item.outcome === 'skipped') {
    return `Автор «${name}»: пропущен — ${notesText(item.notes) || 'нет данных'}.`;
  }
  if (item.outcome === 'added') {
    return `Автор «${name}»: добавлено фильмов — ${item.added}.`;
  }
  return `Автор «${name}»: новых фильмов нет.`;
}

/**
 * Bulk banner: «Обновлено авторов: n из m, добавлено фильмов: k.», the
 * client-side count of selected rows without a QID (not sent), and the
 * first three per-author errors/skips.
 */
export function bulkSyncSummary(
  report: AdminAuthorFilmSyncReport,
  skippedWithoutQid = 0,
  names: Record<string, string> = {}
): string {
  let text = `Обновлено авторов: ${report.updated} из ${report.processed}, добавлено фильмов: ${report.added}.`;
  if (skippedWithoutQid > 0) {
    text += ` Без QID пропущено: ${skippedWithoutQid}.`;
  }
  const details = report.items
    .filter((i) => i.outcome === 'error' || i.outcome === 'skipped')
    .slice(0, 3)
    .map((i) => syncItemText(i, names[i.author_id]));
  if (details.length > 0) {
    text += ` ${details.join(' ')}`;
  }
  return text;
}

/** Banner text for a failed sync call. */
export function syncErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 502) return 'Wikidata недоступна, попробуйте позже.';
    if (err.status === 503) return 'Поиск фильмов об авторах не настроен.';
  }
  return 'Не удалось найти фильмы об авторе.';
}

/** Whether the banner for a report should be styled as an error. */
export function syncReportHasErrors(report: AdminAuthorFilmSyncReport): boolean {
  return report.items.some((i) => i.outcome === 'error');
}
