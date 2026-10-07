import { describe, expect, it } from 'vitest';
import {
  ENRICHMENT_NOTE_TEXTS,
  enrichmentFieldsText,
  enrichmentNotesText,
  importEnrichmentSuffix,
} from './enrichment-text';

describe('enrichment text', () => {
  it('maps every field label', () => {
    expect(enrichmentFieldsText(['birth_date', 'death_date', 'photo'])).toBe('дата рождения, дата смерти, фото');
  });

  it('maps every note and keeps unknown codes', () => {
    expect(enrichmentNotesText(['birth_date_unusable'])).toBe(
      'дата рождения в Wikidata неточная или противоречивая — пропущена'
    );
    expect(enrichmentNotesText(['death_date_unusable'])).toBe(
      'дата смерти в Wikidata неточная или противоречивая — пропущена'
    );
    expect(enrichmentNotesText(['photo_license_not_free'])).toBe('фото под несвободной лицензией — пропущено');
    expect(enrichmentNotesText(['photo_attribution_missing'])).toBe('у фото не указан автор — пропущено');
    expect(enrichmentNotesText(['photo_unavailable'])).toBe('файл фото недоступен на Commons');
    expect(enrichmentNotesText(['entity_not_found'])).toBe('элемент Wikidata не найден');
    expect(enrichmentNotesText(['no_qid'])).toBe('у автора нет QID');
    expect(enrichmentNotesText(['wikidata_unavailable', 'something_new'])).toBe('Wikidata недоступна; something_new');
    expect(Object.keys(ENRICHMENT_NOTE_TEXTS)).toHaveLength(9);
  });

  it('builds the import suffix', () => {
    expect(importEnrichmentSuffix(null)).toBe('');
    expect(importEnrichmentSuffix({ filled: ['birth_date', 'photo'], notes: [], error: '' })).toBe(
      ' Из Wikidata добавлены: дата рождения, фото.'
    );
    expect(importEnrichmentSuffix({ filled: [], notes: ['photo_license_not_free'], error: '' })).toBe(
      ' Из Wikidata ничего не добавлено. Примечания: фото под несвободной лицензией — пропущено.'
    );
    expect(importEnrichmentSuffix({ filled: [], notes: [], error: 'failed to save enrichment' })).toBe(
      ' Данные из Wikidata не сохранены: failed to save enrichment.'
    );
  });
});
