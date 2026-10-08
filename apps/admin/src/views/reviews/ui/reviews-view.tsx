'use client';

import { useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminReviewsApi } from '@/entities/review';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { AdminReview, ApiError } from '@bookhaven/api-client';
import {
  Badge,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Pagination } from '@/shared/ui';
import { ModerateReviewButton } from '@/features/moderate-review';

export function ReviewsView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  const [feedback, setFeedback] = useState<{
    type: 'success' | 'error';
    message: string;
  } | null>(null);

  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const visibility = searchParams.get('visibility') || '';
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'reviews', { visibility, page }],
    queryFn: () =>
      adminReviewsApi.list({
        visibility: visibility as 'visible' | 'hidden' | '',
        page,
        limit,
      }),
  });

  const moderateMutation = useMutation({
    mutationFn: ({ id, isHidden }: { id: string; isHidden: boolean }) =>
      adminReviewsApi.moderate(id, isHidden),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'reviews'] });
      // "Скрытых отзывов" dashboard tile.
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
      setFeedback({
        type: 'success',
        message: variables.isHidden
          ? 'Отзыв скрыт. Счетчик отзывов книги обновлен.'
          : 'Отзыв восстановлен. Счетчик отзывов книги обновлен.',
      });
    },
    onError: (error) => {
      const message =
        error instanceof ApiError
          ? error.message || 'Не удалось обновить отзыв.'
          : 'Не удалось обновить отзыв.';
      setFeedback({ type: 'error', message });
    },
  });

  const updateParams = (updates: Record<string, string | undefined>) => {
    const params = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        params.delete(key);
      } else {
        params.set(key, value);
      }
    });
    router.push(`${pathname}?${params.toString()}` as any);
  };

  const handleModerate = (review: AdminReview) => {
    setFeedback(null);
    const action = review.is_hidden ? 'восстановить' : 'скрыть';
    if (!confirm(`Вы уверены, что хотите ${action} этот отзыв?`)) {
      return;
    }
    moderateMutation.mutate({ id: review.id, isHidden: !review.is_hidden });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Модерация отзывов</h1>

      <Select
        options={[
          { value: '', label: 'Все' },
          { value: 'visible', label: 'Видимые' },
          { value: 'hidden', label: 'Скрытые' },
        ]}
        value={visibility}
        onChange={(e) => updateParams({ visibility: e.target.value, page: '1' })}
        className="w-full md:w-48"
      />

      {feedback && (
        <div
          className={`rounded-md p-4 text-sm ${
            feedback.type === 'success'
              ? 'bg-green-100 text-green-900'
              : 'bg-destructive/10 text-destructive'
          }`}
        >
          {feedback.message}
          <button
            onClick={() => setFeedback(null)}
            className="ml-4 underline"
            type="button"
          >
            Скрыть
          </button>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить отзывы.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : data?.items.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-muted-foreground">
          Отзывы не найдены.
        </div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Книга</TableHead>
                  <TableHead>Пользователь</TableHead>
                  <TableHead>Рейтинг</TableHead>
                  <TableHead>Отзыв</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((review: AdminReview) => (
                  <TableRow key={review.id}>
                    <TableCell className="font-medium">{review.book_title}</TableCell>
                    <TableCell>{review.user?.name || '—'}</TableCell>
                    <TableCell>{review.rating}</TableCell>
                    <TableCell className="max-w-md truncate">
                      {review.content}
                    </TableCell>
                    <TableCell>
                      {review.is_hidden ? (
                        <Badge variant="secondary">Скрыт</Badge>
                      ) : (
                        <Badge variant="default">Видим</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <ModerateReviewButton
                        review={review}
                        onModerate={handleModerate}
                        pending={moderateMutation.isPending}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => updateParams({ page: String(p) })}
          />
        </>
      )}
    </div>
  );
}
