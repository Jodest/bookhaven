'use client';

import { api, Review } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useAuthStore } from '@/lib/auth-store';
import { ReviewForm } from './review-form';

interface ReviewsSectionProps {
  bookId: string;
  slug: string;
}

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

export function ReviewsSection({ bookId, slug }: ReviewsSectionProps) {
  const { isAuthenticated, user } = useAuthStore();
  const [items, setItems] = useState<Review[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(5);
  const [myReview, setMyReview] = useState<Review | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  const fetchReviews = async (targetPage: number, append: boolean) => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.reviews.list(bookId, { page: targetPage, limit });
      if (append) {
        setItems((prev) => [...prev, ...response.items]);
      } else {
        setItems(response.items);
      }
      setTotal(response.total);
      setMyReview(response.my_review);
      setPage(targetPage);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка загрузки рецензий');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setItems([]);
    setTotal(0);
    setPage(1);
    setMyReview(undefined);
    fetchReviews(1, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bookId, isAuthenticated]);

  const handleLoadMore = () => {
    fetchReviews(page + 1, true);
  };

  const handleSuccess = () => {
    setShowForm(false);
    fetchReviews(1, false);
  };

  const handleDelete = async (review: Review) => {
    if (!confirm('Удалить рецензию?')) return;
    try {
      await api.reviews.remove(review.id);
      handleSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления');
    }
  };

  const handleToggleLike = async (review: Review) => {
    if (!isAuthenticated) return;
    try {
      const result = await api.reviews.toggleLike(review.id);
      setItems((prev) =>
        prev.map((r) =>
          r.id === review.id
            ? { ...r, likes_count: result.likes_count, liked_by_me: result.liked }
            : r
        )
      );
      if (myReview && myReview.id === review.id) {
        setMyReview({
          ...myReview,
          likes_count: result.likes_count,
          liked_by_me: result.liked,
        });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка лайка');
    }
  };

  const isOwner = (review: Review) => user && review.user_id === user.id;

  return (
    <section className="mt-10">
      <h2 className="text-2xl font-bold">Рецензии</h2>

      {error && (
        <div className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {isAuthenticated ? (
        <div className="mt-4">
          {!showForm && (
            <Button onClick={() => setShowForm(true)}>
              {myReview ? 'Редактировать рецензию' : 'Написать рецензию'}
            </Button>
          )}
          {showForm && (
            <div className="mt-4">
              <ReviewForm
                bookId={bookId}
                initialReview={myReview}
                onSuccess={handleSuccess}
                onCancel={() => setShowForm(false)}
              />
            </div>
          )}
        </div>
      ) : (
        <div className="mt-4">
          <Button variant="outline" asChild>
            <Link href={`/login?redirect=/books/${slug}`}>Войдите, чтобы написать рецензию</Link>
          </Button>
        </div>
      )}

      <div className="mt-6 space-y-6">
        {items.length === 0 && !loading && (
          <p className="text-muted-foreground">
            Пока нет рецензий — станьте первым.
          </p>
        )}

        {items.map((review) => (
          <article key={review.id} className="rounded-md border p-4">
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
                onClick={() => handleToggleLike(review)}
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

              {isOwner(review) && (
                <div className="ml-auto flex gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setMyReview(review);
                      setShowForm(true);
                    }}
                  >
                    Редактировать
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => handleDelete(review)}
                  >
                    Удалить
                  </Button>
                </div>
              )}
            </div>
          </article>
        ))}

        {loading && <p className="text-sm text-muted-foreground">Загрузка...</p>}

        {items.length < total && !loading && (
          <Button variant="outline" onClick={handleLoadMore}>
            Показать ещё
          </Button>
        )}
      </div>
    </section>
  );
}
