'use client';

import { api } from '@bookhaven/api-client';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useId, useState } from 'react';
import { useAuthStore } from '@/entities/session';
import { loginHref } from '@/shared/lib/safe-redirect';

interface BookRatingProps {
  bookId: string;
  slug: string;
}

function Star({ fillPercent, className = 'h-6 w-6' }: { fillPercent: number; className?: string }) {
  // useId output contains ':' which is awkward inside url(#...) references.
  const id = `${useId().replace(/:/g, '')}-star`;
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
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

function InteractiveStars({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  const [hoverValue, setHoverValue] = useState(0);
  const shown = hoverValue || value;
  return (
    <div className="flex md:justify-center" onMouseLeave={() => setHoverValue(0)}>
      {[1, 2, 3, 4, 5].map((i) => (
        <button
          key={i}
          type="button"
          onClick={() => onChange(i)}
          onMouseEnter={() => setHoverValue(i)}
          className="rounded p-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={`Оценить на ${i}`}
        >
          <Star fillPercent={i <= shown ? 100 : 0} className="h-8 w-8" />
        </button>
      ))}
    </div>
  );
}

export function BookRating({ bookId, slug }: BookRatingProps) {
  const { isAuthenticated } = useAuthStore();
  const router = useRouter();
  const [myRating, setMyRating] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
      await api.ratings.set(bookId, value);
      // Re-render the server-side rating summary with the new average.
      router.refresh();
    } catch (err) {
      setMyRating(previousMyRating);
      setError(err instanceof Error ? err.message : 'Ошибка сохранения оценки');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-2 md:text-center">
      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {isAuthenticated ? (
        <>
          <p className="text-sm text-muted-foreground">
            {myRating > 0 ? `Моя оценка: ${myRating}` : 'Оцените книгу'}
          </p>
          <InteractiveStars value={myRating} onChange={handleSetRating} />
        </>
      ) : (
        <Link
          href={loginHref(`/books/${slug}`)}
          className="block text-sm text-primary hover:underline"
        >
          Войдите, чтобы оценить
        </Link>
      )}
    </div>
  );
}
