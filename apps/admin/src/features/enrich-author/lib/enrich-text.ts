import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthorEnrichmentItem, AdminAuthorEnrichmentReport } from '@bookhaven/api-client';
import { enrichmentFieldsText, enrichmentNotesText } from '@/entities/author';

/** Result line for one author (single-row action and bulk details). */
export function enrichItemText(item: AdminAuthorEnrichmentItem, fallbackName = ''): string {
  const name = item.name || fallbackName || item.author_id;
  const notes = item.notes.length > 0 ? enrichmentNotesText(item.notes) : '';

  if (item.outcome === 'error') {
    const reason = notes || item.message || 'неизвестная ошибка';
    return `Автор «${name}»: не удалось обогатить — ${reason}.`;
  }
  if (item.outcome === 'skipped') {
    return `Автор «${name}»: пропущен — ${notes || 'нет данных'}.`;
  }
  let text =
    item.filled.length > 0
      ? `Автор «${name}»: добавлены ${enrichmentFieldsText(item.filled)}.`
      : `Автор «${name}»: нечего добавить — поля уже заполнены или в Wikidata нет подходящих данных.`;
  if (notes) {
    text += ` Примечания: ${notes}.`;
  }
  return text;
}

/**
 * Bulk banner: «Обогащено X из N», the client-side count of selected rows
 * without a QID (not sent), and the first three per-author errors/notes.
 */
export function bulkEnrichSummary(
  report: AdminAuthorEnrichmentReport,
  skippedWithoutQid = 0,
  names: Record<string, string> = {}
): string {
  let text = `Обогащено ${report.enriched} из ${report.processed}.`;
  if (skippedWithoutQid > 0) {
    text += ` Без QID пропущено: ${skippedWithoutQid}.`;
  }
  const details = report.items
    .filter((i) => i.outcome === 'error' || i.outcome === 'skipped' || i.notes.length > 0)
    .slice(0, 3)
    .map((i) => enrichItemText(i, names[i.author_id]));
  if (details.length > 0) {
    text += ` ${details.join(' ')}`;
  }
  return text;
}

/** Banner text for a failed enrichment call. */
export function enrichErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 502) return 'Wikidata недоступна, попробуйте позже.';
    if (err.status === 503) return 'Обогащение не настроено.';
    if (err.message) return err.message;
  }
  return 'Не удалось обогатить авторов.';
}

/** Whether the banner for a report should be styled as an error. */
export function enrichReportHasErrors(report: AdminAuthorEnrichmentReport): boolean {
  return report.items.some((i) => i.outcome === 'error');
}
