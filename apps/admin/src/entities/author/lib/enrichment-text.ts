import type { AdminAuthorImportEnrichment } from '@bookhaven/api-client';

// Russian labels for Wikidata enrichment results
// (docs/features/author-wikidata-enrichment.md, UX). Shared by
// features/enrich-author and features/import-wikidata-author.

export const ENRICHMENT_FIELD_LABELS: Record<string, string> = {
  birth_date: 'дата рождения',
  death_date: 'дата смерти',
  photo: 'фото',
};

export const ENRICHMENT_NOTE_TEXTS: Record<string, string> = {
  birth_date_unusable: 'дата рождения в Wikidata неточная или противоречивая — пропущена',
  death_date_unusable: 'дата смерти в Wikidata неточная или противоречивая — пропущена',
  photo_license_not_free: 'фото под несвободной лицензией — пропущено',
  photo_attribution_missing: 'у фото не указан автор — пропущено',
  photo_unavailable: 'файл фото недоступен на Commons',
  entity_not_found: 'элемент Wikidata не найден',
  no_qid: 'у автора нет QID',
  wikidata_unavailable: 'Wikidata недоступна',
  commons_unavailable: 'Commons недоступен — фото не загружено',
};

/** "дата рождения, фото" for filled field codes (unknown codes as-is). */
export function enrichmentFieldsText(filled: string[]): string {
  return filled.map((f) => ENRICHMENT_FIELD_LABELS[f] ?? f).join(', ');
}

/** Note texts joined with "; " (unknown codes as-is). */
export function enrichmentNotesText(notes: string[]): string {
  return notes.map((n) => ENRICHMENT_NOTE_TEXTS[n] ?? n).join('; ');
}

/**
 * Suffix appended to the import banner of a created author, e.g.
 * " Из Wikidata добавлены: дата рождения, фото." Empty for a null
 * enrichment.
 */
export function importEnrichmentSuffix(enrichment: AdminAuthorImportEnrichment | null): string {
  if (!enrichment) return '';
  if (enrichment.error) {
    return ` Данные из Wikidata не сохранены: ${enrichment.error}.`;
  }
  let text =
    enrichment.filled.length > 0
      ? ` Из Wikidata добавлены: ${enrichmentFieldsText(enrichment.filled)}.`
      : ' Из Wikidata ничего не добавлено.';
  if (enrichment.notes.length > 0) {
    text += ` Примечания: ${enrichmentNotesText(enrichment.notes)}.`;
  }
  return text;
}
