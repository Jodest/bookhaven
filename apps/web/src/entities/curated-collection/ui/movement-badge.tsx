import type { CollectionMovement } from '@bookhaven/api-client';
import { formatMovement } from '../lib/movement';

interface MovementBadgeProps {
  movement: CollectionMovement | null | undefined;
}

/** «↑3» / «↓2» / «Новинка» next to a place number; renders nothing for unchanged places. */
export function MovementBadge({ movement }: MovementBadgeProps) {
  const view = formatMovement(movement);
  if (!view) return null;
  const color =
    movement?.direction === 'up'
      ? 'text-green-600'
      : movement?.direction === 'down'
        ? 'text-red-600'
        : 'text-primary';
  return (
    <span className={`text-xs font-semibold ${color}`} aria-label={view.label} title={view.label}>
      {view.text}
    </span>
  );
}
