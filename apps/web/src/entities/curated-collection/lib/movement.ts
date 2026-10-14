import type { CollectionMovement } from '@bookhaven/api-client';

export interface MovementView {
  /** Visible text: «↑3», «↓2» or «Новинка». */
  text: string;
  /** Accessible description. */
  label: string;
}

/** Visible movement of a place; null when nothing should be shown (no baseline or unchanged). */
export function formatMovement(movement: CollectionMovement | null | undefined): MovementView | null {
  if (!movement) return null;
  switch (movement.direction) {
    case 'up':
      return { text: `↑${movement.by}`, label: `Поднялась на ${movement.by}` };
    case 'down':
      return { text: `↓${movement.by}`, label: `Опустилась на ${movement.by}` };
    case 'new':
      return { text: 'Новинка', label: 'Новинка в подборке' };
    default:
      return null;
  }
}
