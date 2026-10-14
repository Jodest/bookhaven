import { describe, expect, it } from 'vitest';
import { pasteStatusLabel, PASTE_BOOK_STATUS_LABELS, SKIP_REASON_LABELS } from './status-labels';

describe('pasteStatusLabel', () => {
  it('labels every status in Russian', () => {
    expect(pasteStatusLabel({ status: 'found', duplicate_of: null })).toBe('Найдено');
    expect(pasteStatusLabel({ status: 'ambiguous', duplicate_of: null })).toBe('Неоднозначно');
    expect(pasteStatusLabel({ status: 'not_found', duplicate_of: null })).toBe(
      'Не найдено — сначала добавьте книгу в каталог'
    );
    expect(pasteStatusLabel({ status: 'rejected', duplicate_of: null })).toBe('Отклонена — нельзя добавить');
    expect(pasteStatusLabel({ status: 'in_collection', duplicate_of: null })).toBe('Уже в подборке');
    expect(pasteStatusLabel({ status: 'invalid', duplicate_of: null })).toBe('Не распознано');
  });

  it('names the earlier line of a duplicate', () => {
    expect(pasteStatusLabel({ status: 'duplicate', duplicate_of: 2 })).toBe('Повтор строки 2');
  });

  it('has book status and skip reason labels', () => {
    expect(PASTE_BOOK_STATUS_LABELS.published).toBe('Опубликована');
    expect(PASTE_BOOK_STATUS_LABELS.pending).toBe('На модерации');
    expect(PASTE_BOOK_STATUS_LABELS.rejected).toBe('Отклонена');
    expect(SKIP_REASON_LABELS.already_in_collection).toBe('уже в подборке');
  });
});
