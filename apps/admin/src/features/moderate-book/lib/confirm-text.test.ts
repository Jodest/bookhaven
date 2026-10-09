import { describe, expect, it } from 'vitest';
import {
  bookStatusConfirmText,
  bulkBookStatusConfirmText,
  bulkBookStatusSummary,
} from './confirm-text';

describe('book moderation confirm text', () => {
  it('row confirm for every status', () => {
    expect(bookStatusConfirmText('published', 'Война и мир')).toBe('Опубликовать книгу «Война и мир»?');
    expect(bookStatusConfirmText('rejected', 'Война и мир')).toBe('Отклонить книгу «Война и мир»?');
    expect(bookStatusConfirmText('pending', 'Война и мир')).toBe('Вернуть книгу «Война и мир» на модерацию?');
  });

  it('bulk confirm for every status', () => {
    expect(bulkBookStatusConfirmText('published', 2)).toBe('Опубликовать выбранные книги (2)?');
    expect(bulkBookStatusConfirmText('rejected', 3)).toBe('Отклонить выбранные книги (3)?');
    expect(bulkBookStatusConfirmText('pending', 4)).toBe('Вернуть выбранные книги на модерацию (4)?');
  });

  it('summary without errors', () => {
    expect(bulkBookStatusSummary({ processed: 2, updated: 2, errors: [] })).toBe('Обновлено 2 из 2.');
  });

  it('summary with errors: title, then names, then id; first three only', () => {
    const text = bulkBookStatusSummary(
      {
        processed: 5,
        updated: 1,
        errors: [
          { book_id: 'b1', title: 'Т1', message: 'm1' },
          { book_id: 'b2', title: '', message: 'm2' },
          { book_id: 'b3', title: '', message: 'm3' },
          { book_id: 'b4', title: 'Т4', message: 'm4' },
        ],
      },
      { b2: 'Из имён' }
    );
    expect(text).toBe('Обновлено 1 из 5. Ошибок: 4 («Т1»: m1; «Из имён»: m2; «b3»: m3).');
  });
});
