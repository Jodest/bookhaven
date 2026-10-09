import type { AdminBookStatus } from '@bookhaven/api-client';
import { Badge, cn } from '@bookhaven/ui';

export const BOOK_STATUS_LABELS: Record<AdminBookStatus, string> = {
  pending: 'На модерации',
  published: 'Опубликована',
  rejected: 'Отклонена',
};

interface BookStatusBadgeProps {
  status: AdminBookStatus;
  className?: string;
}

/**
 * Book moderation status badge (see docs/features/book-status-model.md):
 * pending = warning (amber), rejected = destructive, published = default.
 */
export function BookStatusBadge({ status, className }: BookStatusBadgeProps) {
  if (status === 'pending') {
    return (
      <Badge
        variant="outline"
        className={cn('border-transparent bg-amber-100 text-amber-900', className)}
      >
        {BOOK_STATUS_LABELS.pending}
      </Badge>
    );
  }
  return (
    <Badge variant={status === 'rejected' ? 'destructive' : 'default'} className={className}>
      {BOOK_STATUS_LABELS[status] ?? status}
    </Badge>
  );
}
