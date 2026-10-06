import type { AdminAuthorStatus } from '@bookhaven/api-client';
import { Badge, cn } from '@bookhaven/ui';

export const AUTHOR_STATUS_LABELS: Record<AdminAuthorStatus, string> = {
  pending: 'На модерации',
  published: 'Опубликован',
  rejected: 'Отклонён',
};

interface AuthorStatusBadgeProps {
  status: AdminAuthorStatus;
  className?: string;
}

/**
 * Author moderation status badge (see docs/features/author-status.md):
 * pending = warning (amber), rejected = destructive, published = default.
 */
export function AuthorStatusBadge({ status, className }: AuthorStatusBadgeProps) {
  if (status === 'pending') {
    return (
      <Badge
        variant="outline"
        className={cn('border-transparent bg-amber-100 text-amber-900', className)}
      >
        {AUTHOR_STATUS_LABELS.pending}
      </Badge>
    );
  }
  return (
    <Badge variant={status === 'rejected' ? 'destructive' : 'default'} className={className}>
      {AUTHOR_STATUS_LABELS[status] ?? status}
    </Badge>
  );
}
