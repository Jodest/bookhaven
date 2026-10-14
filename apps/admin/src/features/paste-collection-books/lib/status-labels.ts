import type { AdminBookStatus, BulkSkipReason, PasteLine, PasteLineStatus } from '@bookhaven/api-client';

/** Row status labels of the paste preview table. `duplicate` is built by pasteStatusLabel. */
export const PASTE_STATUS_LABELS: Record<PasteLineStatus, string> = {
  found: 'Найдено',
  ambiguous: 'Неоднозначно',
  not_found: 'Не найдено — сначала добавьте книгу в каталог',
  rejected: 'Отклонена — нельзя добавить',
  in_collection: 'Уже в подборке',
  duplicate: 'Повтор',
  invalid: 'Не распознано',
};

/** Short names of the statuses for the counts summary. */
export const PASTE_COUNT_LABELS: Record<PasteLineStatus, string> = {
  found: 'Найдено',
  ambiguous: 'Неоднозначно',
  not_found: 'Не найдено',
  rejected: 'Отклонено',
  in_collection: 'Уже в подборке',
  duplicate: 'Повторы',
  invalid: 'Не распознано',
};

export const PASTE_COUNT_ORDER: PasteLineStatus[] = [
  'found',
  'ambiguous',
  'not_found',
  'rejected',
  'in_collection',
  'duplicate',
  'invalid',
];

export function pasteStatusLabel(line: Pick<PasteLine, 'status' | 'duplicate_of'>): string {
  if (line.status === 'duplicate' && line.duplicate_of !== null) {
    return `Повтор строки ${line.duplicate_of}`;
  }
  return PASTE_STATUS_LABELS[line.status];
}

export const PASTE_BOOK_STATUS_LABELS: Record<AdminBookStatus, string> = {
  published: 'Опубликована',
  pending: 'На модерации',
  rejected: 'Отклонена',
};

export const SKIP_REASON_LABELS: Record<BulkSkipReason, string> = {
  not_found: 'книги нет в каталоге',
  rejected: 'книга отклонена',
  already_in_collection: 'уже в подборке',
  duplicate: 'повтор в списке',
};
