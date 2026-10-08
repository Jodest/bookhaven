import type { AdminGenreStatus } from '@bookhaven/api-client';
import { Badge, cn } from '@bookhaven/ui';

export const GENRE_STATUS_LABELS: Record<AdminGenreStatus, string> = {
  pending: 'На модерации',
  published: 'Опубликован',
  rejected: 'Отклонён',
};

interface GenreStatusBadgeProps {
  status: AdminGenreStatus;
  className?: string;
}

/**
 * Genre moderation status badge (see docs/features/genre-status.md):
 * pending = warning (amber), rejected = destructive, published = default.
 */
export function GenreStatusBadge({ status, className }: GenreStatusBadgeProps) {
  if (status === 'pending') {
    return (
      <Badge
        variant="outline"
        className={cn('border-transparent bg-amber-100 text-amber-900', className)}
      >
        {GENRE_STATUS_LABELS.pending}
      </Badge>
    );
  }
  return (
    <Badge variant={status === 'rejected' ? 'destructive' : 'default'} className={className}>
      {GENRE_STATUS_LABELS[status] ?? status}
    </Badge>
  );
}
