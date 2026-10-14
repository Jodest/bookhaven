import type { AdminCuratedItemState } from '@bookhaven/api-client';

/** Marker text of an item that holds no public place; ranked items show their place. */
export const ITEM_STATE_LABELS: Record<Exclude<AdminCuratedItemState, 'ranked'>, string> = {
  reserve: 'Резерв',
  hidden: 'Скрыта: не опубликована',
};

/**
 * Warning shown when fewer books are visible than the collection has places;
 * null when the collection is full.
 */
export function reserveWarning(shown: number, size: number): string | null {
  if (shown >= size) return null;
  return `В подборке видно ${shown} из ${size} книг — пополните резерв`;
}
