'use client';

import { AdminReview } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';

interface ModerateReviewButtonProps {
  review: AdminReview;
  onModerate: (review: AdminReview) => void;
  pending: boolean;
}

export function ModerateReviewButton({ review, onModerate, pending }: ModerateReviewButtonProps) {
  return (
    <Button
      variant={review.is_hidden ? 'outline' : 'destructive'}
      size="sm"
      onClick={() => onModerate(review)}
      disabled={pending}
    >
      {review.is_hidden ? 'Восстановить' : 'Скрыть'}
    </Button>
  );
}
