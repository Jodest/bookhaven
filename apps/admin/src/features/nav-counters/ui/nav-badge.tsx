import { Badge } from '@bookhaven/ui';

interface NavBadgeProps {
  count: number | undefined;
  /** Screen-reader prefix; rendered as `, {srLabel}: {count}`. */
  srLabel: string;
  variant?: 'default' | 'secondary' | 'destructive' | 'outline';
  className?: string;
}

/**
 * Counter badge for navigation links. Renders nothing for 0/undefined.
 * The visual value is capped at 99+ and hidden from assistive tech; the exact
 * count is exposed via sr-only text, so the link name reads e.g.
 * "Модерация, ожидают проверки: 11".
 */
export function NavBadge({ count, srLabel, variant = 'secondary', className }: NavBadgeProps) {
  if (!count) return null;
  return (
    <>
      <Badge variant={variant} className={className} aria-hidden="true">
        {count > 99 ? '99+' : count}
      </Badge>
      <span className="sr-only">{`, ${srLabel}: ${count}`}</span>
    </>
  );
}
