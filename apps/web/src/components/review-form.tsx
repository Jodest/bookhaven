'use client';

import { api, ApiError, Review } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

const reviewSchema = z.object({
  rating: z.number().min(1, 'Выберите оценку').max(5),
  content: z
    .string()
    .transform((value) => value.trim())
    .pipe(
      z
        .string()
        .min(1, 'Рецензия не может быть пустой')
        .max(10000, 'Рецензия слишком длинная')
    ),
});

type ReviewFormData = z.infer<typeof reviewSchema>;

interface ReviewFormProps {
  bookId: string;
  initialReview?: Review;
  onSuccess: () => void;
  onCancel?: () => void;
}

function Star({ filled }: { filled: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={`h-6 w-6 ${filled ? 'text-yellow-400' : 'text-muted-foreground/30'}`}
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
    </svg>
  );
}

export function ReviewForm({ bookId, initialReview, onSuccess, onCancel }: ReviewFormProps) {
  const isEdit = Boolean(initialReview);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<ReviewFormData>({
    resolver: zodResolver(reviewSchema),
    defaultValues: {
      rating: initialReview?.rating ?? 0,
      content: initialReview?.content ?? '',
    },
  });

  const rating = watch('rating');

  useEffect(() => {
    if (initialReview) {
      reset({
        rating: initialReview.rating,
        content: initialReview.content,
      });
    }
  }, [initialReview, reset]);

  const onSubmit = async (data: ReviewFormData) => {
    setSubmitError(null);
    try {
      if (isEdit && initialReview) {
        const payload: { rating?: number; content?: string } = {};
        if (data.rating !== initialReview.rating) payload.rating = data.rating;
        if (data.content !== initialReview.content) payload.content = data.content;
        if (Object.keys(payload).length > 0) {
          await api.reviews.update(initialReview.id, payload);
        }
      } else {
        await api.reviews.create(bookId, data);
      }
      onSuccess();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409 && !isEdit) {
        onSuccess();
        return;
      }
      const message = err instanceof Error ? err.message : 'Ошибка сохранения';
      setSubmitError(message);
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 rounded-md border p-4">
      {submitError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {submitError}
        </div>
      )}

      <div>
        <label className="mb-1 block text-sm font-medium">Оценка</label>
        <div className="flex">
          {[1, 2, 3, 4, 5].map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => setValue('rating', value, { shouldValidate: true })}
              className="p-0.5 focus:outline-none"
              aria-label={`Оценка ${value}`}
            >
              <Star filled={value <= rating} />
            </button>
          ))}
        </div>
        {errors.rating && (
          <p className="mt-1 text-sm text-destructive">{errors.rating.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="review-content" className="mb-1 block text-sm font-medium">
          Рецензия
        </label>
        <textarea
          id="review-content"
          rows={5}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
          placeholder="Напишите, что вам понравилось или не понравилось..."
          {...register('content')}
        />
        {errors.content && (
          <p className="mt-1 text-sm text-destructive">{errors.content.message}</p>
        )}
      </div>

      <div className="flex gap-2">
        <Button type="submit" disabled={isSubmitting}>
          {isEdit ? 'Сохранить' : 'Опубликовать'}
        </Button>
        {onCancel && (
          <Button type="button" variant="outline" onClick={onCancel} disabled={isSubmitting}>
            Отмена
          </Button>
        )}
      </div>
    </form>
  );
}
