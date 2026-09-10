'use client';

import { ReviewCard, reviewsApi } from '@/entities/review';
import { useAuthStore } from '@/entities/session';
import { ReviewForm } from '@/features/write-review';
import { Review } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useEffect, useState } from 'react';

interface ReviewsSectionProps {
  bookId: string;
  slug: string;
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
      const response = await reviewsApi.list(bookId, { page: targetPage, limit });
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
      await reviewsApi.remove(review.id);
      handleSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления');
    }
  };

  const handleToggleLike = async (review: Review) => {
    if (!isAuthenticated) return;
    try {
      const result = await reviewsApi.toggleLike(review.id);
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

  const isOwner = (review: Review) => Boolean(user && review.user_id === user.id);

  const handleEdit = (review: Review) => {
    setMyReview(review);
    setShowForm(true);
  };

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
          <ReviewCard
            key={review.id}
            review={review}
            isOwner={isOwner(review)}
            isAuthenticated={isAuthenticated}
            onToggleLike={handleToggleLike}
            onEdit={handleEdit}
            onDelete={handleDelete}
          />
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
