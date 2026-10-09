import type { ModerationQueueColumn } from '../model/types';

/** Shared "Добавлен" column: the queue item's creation date in ru-RU format. */
export function moderationCreatedAtColumn<TItem>(
  get: (item: TItem) => string
): ModerationQueueColumn<TItem> {
  return {
    id: 'created_at',
    header: 'Добавлен',
    cellClassName: 'align-top whitespace-nowrap text-sm',
    cell: (item) => new Date(get(item)).toLocaleDateString('ru-RU'),
  };
}
