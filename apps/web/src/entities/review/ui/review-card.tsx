'use client';

import { Review } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';

function formatDate(dateString: string) {
  return new Date(dateString).toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

function Star({ filled }: { filled: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={`h-4 w-4 ${filled ? 'text-yellow-400' : 'text-muted-foreground/30'}`}
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
    </svg>
  );
}

function StarRating({ value }: { value: number }) {
  return (
    <div className="flex">
      {[1, 2, 3, 4, 5].map((i) => (
        <Star key={i} filled={i <= value} />
      ))}
    </div>
  );
}

function Avatar({ name, url }: { name: string | null; url: string | null }) {
  const initial = name ? name.charAt(0).toUpperCase() : '?';
  return (
    <div className="flex h-10 w-10 items-center justify-center overflow-hidden rounded-full bg-muted text-sm font-medium">
      {url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={url} alt={name ?? ''} className="h-full w-full object-cover" />
      ) : (
        initial
      )}
    </div>
  );
}

interface ReviewCardProps {
  review: Review;
  isOwner: boolean;
  isAuthenticated: boolean;
  onToggleLike: (review: Review) => void;
  onEdit: (review: Review) => void;
  onDelete: (review: Review) => void;
}

export function ReviewCard({
  review,
  isOwner,
  isAuthenticated,
  onToggleLike,
  onEdit,
  onDelete,
}: ReviewCardProps) {
  return (
    <article className="rounded-md border p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <Avatar name={review.user.name} url={review.user.avatar_url} />
          <div>
            <p className="font-medium">{review.user.name ?? 'Аноним'}</p>
            <p className="text-xs text-muted-foreground">{formatDate(review.created_at)}</p>
          </div>
        </div>
        <StarRating value={review.rating} />
      </div>

      <p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed">{review.content}</p>

      <div className="mt-4 flex items-center gap-4">
        <button
          type="button"
          onClick={() => onToggleLike(review)}
          className={`flex items-center gap-1 text-sm ${
            review.liked_by_me ? 'text-primary' : 'text-muted-foreground'
          }`}
          disabled={!isAuthenticated}
        >
          <svg
            viewBox="0 0 24 24"
            className="h-4 w-4"
            fill={review.liked_by_me ? 'currentColor' : 'none'}
            stroke="currentColor"
            strokeWidth="2"
          >
            <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
          </svg>
          {review.likes_count}
        </button>

        {isOwner && (
          <div className="ml-auto flex gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={() => onEdit(review)}>
              Редактировать
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => onDelete(review)}>
              Удалить
            </Button>
          </div>
        )}
      </div>
    </article>
  );
}
