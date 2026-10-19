'use client';

import { readingChallengesApi, readingChallengeKeys } from '@/entities/reading-challenge';
import { isApiError, ReadingChallenge } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

const goalSchema = z.object({
  goal: z.coerce
    .number({ invalid_type_error: 'Введите целое число' })
    .int('Цель должна быть целым числом')
    .min(1, 'Минимум 1 книга')
    .max(10000, 'Максимум 10000 книг'),
});

type GoalForm = z.infer<typeof goalSchema>;

interface ManageReadingChallengeGoalProps {
  year: number;
  challenge: ReadingChallenge | null;
  onSaved: (updated: ReadingChallenge) => void;
}

function pluralizeBooks(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'книга';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'книги';
  return 'книг';
}

function mapErrorToRussian(err: unknown): string {
  if (isApiError(err)) {
    const message = err.message.toLowerCase();
    if (message.includes('goal')) {
      return 'Цель должна быть целым числом от 1 до 10000';
    }
    if (message.includes('year')) {
      return 'Некорректный год';
    }
    if (err.status === 401) {
      return 'Необходимо войти в аккаунт';
    }
  }
  return 'Не удалось сохранить цель. Попробуйте позже.';
}

export function ManageReadingChallengeGoal({
  year,
  challenge,
  onSaved,
}: ManageReadingChallengeGoalProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<GoalForm>({
    resolver: zodResolver(goalSchema),
  });

  useEffect(() => {
    setIsEditing(false);
    setSaveError(null);
  }, [year]);

  const onSaveGoal = async (data: GoalForm) => {
    setSaveError(null);
    try {
      const updated = await readingChallengesApi.upsert(year, { goal: data.goal });
      void queryClient.invalidateQueries({ queryKey: readingChallengeKeys.year(year) });
      setIsEditing(false);
      reset();
      onSaved(updated);
    } catch (err) {
      setSaveError(mapErrorToRussian(err));
    }
  };

  const startEditing = () => {
    reset({ goal: challenge?.goal ?? undefined });
    setIsEditing(true);
    setSaveError(null);
  };

  return (
    <div className="mt-6 max-w-xl rounded-lg border bg-card p-6 text-card-foreground shadow-sm">
      {challenge?.goal != null ? (
        <>
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">{year} год</h2>
            {!isEditing && (
              <Button variant="outline" size="sm" onClick={startEditing}>
                Изменить цель
              </Button>
            )}
          </div>

          <p className="mt-3 text-sm text-muted-foreground">
            Прочитано {challenge.booksRead} из {challenge.goal} {pluralizeBooks(challenge.goal)}
            {challenge.progressPercent != null ? ` (${challenge.progressPercent}%)` : ''}
          </p>

          <div className="mt-2 h-3 w-full overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-primary transition-all"
              style={{ width: `${challenge.progressPercent ?? 0}%` }}
            />
          </div>

          {challenge.completed && (
            <div className="mt-3 rounded-md bg-green-100 p-3 text-sm text-green-800">
              Цель на {year} год достигнута!
            </div>
          )}
        </>
      ) : (
        <>
          <h2 className="text-xl font-semibold">{year} год</h2>
          <p className="mt-3 text-sm text-muted-foreground">
            Цель на {year} год пока не задана. Вы уже прочитали {challenge?.booksRead ?? 0}{' '}
            {pluralizeBooks(challenge?.booksRead ?? 0)}. Задайте цель, чтобы отслеживать прогресс.
          </p>
        </>
      )}

      {(isEditing || challenge?.goal == null) && (
        <form onSubmit={handleSubmit(onSaveGoal)} className="mt-4 flex gap-2">
          <div className="flex-1">
            <input
              type="number"
              min={1}
              max={10000}
              placeholder="Например, 20"
              {...register('goal')}
              className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
            />
            {errors.goal && (
              <p className="mt-1 text-sm text-destructive">{errors.goal.message}</p>
            )}
          </div>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Сохранение...' : 'Сохранить'}
          </Button>
          {isEditing && challenge?.goal != null && (
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setIsEditing(false);
                setSaveError(null);
              }}
            >
              Отмена
            </Button>
          )}
        </form>
      )}

      {saveError && (
        <div className="mt-3 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {saveError}
        </div>
      )}

      <p className="mt-4 text-xs text-muted-foreground">
        Прогресс считается по книгам на полке «Прочитано» за этот год. Если книга позже убрана
        с полки, она больше не учитывается.
      </p>
    </div>
  );
}
