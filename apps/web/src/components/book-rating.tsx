'use client';

import { api } from '@bookhaven/api-client';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useAuthStore } from '@/lib/auth-store';

interface BookRatingProps {
  bookId: string;
  slug: string;
  rating: number;
  ratingsCount: number;
}

function Star({ fillPercent }: { fillPercent: number }) {
  const id = `${useId()}-star`;
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-6 w-6"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <clipPath id={`${id}-clip`}>
          <rect x="0" y="0" width={`${fillPercent}%`} height="100%" />
        </clipPath>
        <path
          id={`${id}-path`}
          d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"
        />
      </defs>
      <use href={`#${id}-path`} className="text-muted-foreground/30" fill="currentColor" />
      <use
        href={`#${id}-path`}
        className="text-yellow-400"
        fill="currentColor"
        clipPath={`url(#${id}-clip)`}
      />
    </svg>
  );
}

function useId() {
  const [id] = useState(() => Math.random().toString(36).slice(2));
  return id;
}

function StarRating({ value }: { value: number }) {
  return (
    <div className="flex">
      {[1, 2, 3, 4, 5].map((i) => {
        const fill = Math.max(0, Math.min(100, (value - (i - 1)) * 100));
        return <Star key={i} fillPercent={fill} />;
      })}
    </div>
  );
}

function InteractiveStars({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  return (
    <div className="flex">
      {[1, 2, 3, 4, 5].map((i) => (
        <button
          key={i}
          type="button"
          onClick={() => onChange(i)}
          className="p-0.5 focus:outline-none"
          aria-label={`Оценить на ${i}`}
        >
          <Star fillPercent={i <= value ? 100 : 0} />
        </button>
      ))}
    </div>
  );
}

export function BookRating({ bookId, slug, rating, ratingsCount }: BookRatingProps) {
  const { isAuthenticated } = useAuthStore();
  const [displayRating, setDisplayRating] = useState(rating);
  const [displayCount, setDisplayCount] = useState(ratingsCount);
  const [myRating, setMyRating] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setDisplayRating(rating);
    setDisplayCount(ratingsCount);
  }, [rating, ratingsCount]);

  useEffect(() => {
    if (!isAuthenticated) return;

    let cancelled = false;
    api.ratings
      .my(bookId)
      .then((res) => {
        if (!cancelled && res) {
          setMyRating(res.value);
        }
      })
      .catch(() => {
        // ignore
      });

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, bookId]);

  const handleSetRating = async (value: number) => {
    if (!isAuthenticated || loading) return;

    const previousMyRating = myRating;
    setMyRating(value);
    setLoading(true);
    setError(null);

    try {
      const result = await api.ratings.set(bookId, value);
      setDisplayRating(result.rating);
      setDisplayCount(result.ratings_count);
    } catch (err) {
      setMyRating(previousMyRating);
      setError(err instanceof Error ? err.message : 'Ошибка сохранения оценки');
    } finally {
      setLoading(false);
    }
  };

  const roundedRating = Math.round(displayRating * 2) / 2;

  return (
    <div className="space-y-2">
      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        {isAuthenticated ? (
          <StarRating value={roundedRating} />
        ) : (
          <Link href={`/login?redirect=/books/${slug}`} className="flex">
            <StarRating value={roundedRating} />
          </Link>
        )}

        <div className="text-sm text-muted-foreground">
          <span className="font-semibold text-foreground">{displayRating.toFixed(1)}</span>
          {' · '}
          {displayCount} оценок
        </div>
      </div>

      {isAuthenticated ? (
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Моя оценка:</span>
          <InteractiveStars value={myRating} onChange={handleSetRating} />
        </div>
      ) : (
        <Link
          href={`/login?redirect=/books/${slug}`}
          className="text-sm text-primary hover:underline"
        >
          Войдите, чтобы оценить
        </Link>
      )}
    </div>
  );
}
