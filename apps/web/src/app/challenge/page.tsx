'use client';

import { useAuthStore } from '@/lib/auth-store';
import { api, isApiError, ReadingChallenge } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import { useEffect, useMemo, useState } from 'react';
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

const currentYear = new Date().getUTCFullYear();

export default function ChallengePage() {
  const { user, isAuthenticated, isLoading: authLoading } = useAuthStore();

  const [selectedYear, setSelectedYear] = useState(currentYear);
  const [challenge, setChallenge] = useState<ReadingChallenge | null>(null);
  const [history, setHistory] = useState<ReadingChallenge[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<GoalForm>({
    resolver: zodResolver(goalSchema),
  });

  const fetchHistory = async () => {
    const response = await api.readingChallenges.list();
    setHistory(response.items);
  };

  const fetchYear = async (year: number) => {
    const data = await api.readingChallenges.get(year);
    setChallenge(data);
    return data;
  };

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    (async () => {
      try {
        const [data] = await Promise.all([fetchYear(selectedYear), fetchHistory()]);
        if (!cancelled) {
          setChallenge(data);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError('Не удалось загрузить читательский вызов. Попробуйте позже.');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isAuthenticated, selectedYear]);

  useEffect(() => {
    setIsEditing(false);
    setSaveError(null);
  }, [selectedYear]);

  const availableYears = useMemo(() => {
    const years = new Set<number>();
    years.add(currentYear);
    years.add(selectedYear);
    for (const item of history) {
      years.add(item.year);
    }
    return Array.from(years).sort((a, b) => b - a);
  }, [history, selectedYear]);

  const onSaveGoal = async (data: GoalForm) => {
    setSaveError(null);
    try {
      const updated = await api.readingChallenges.upsert(selectedYear, { goal: data.goal });
      setChallenge(updated);
      setIsEditing(false);
      reset();
      await fetchHistory();
    } catch (err) {
      setSaveError(mapErrorToRussian(err));
    }
  };

  const startEditing = () => {
    reset({ goal: challenge?.goal ?? undefined });
    setIsEditing(true);
    setSaveError(null);
  };

  if (authLoading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Необходимо войти в аккаунт, чтобы задать читательский вызов.</p>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold tracking-tight">Читательский вызов</h1>

      <div className="mt-6 flex items-center gap-3">
        <label htmlFor="year" className="text-sm font-medium text-muted-foreground">
          Год
        </label>
        <select
          id="year"
          value={selectedYear}
          onChange={(e) => setSelectedYear(Number(e.target.value))}
          className="rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
        >
          {availableYears.map((year) => (
            <option key={year} value={year}>
              {year}
            </option>
          ))}
        </select>
      </div>

      {loading ? (
        <p className="mt-6 text-muted-foreground">Загрузка вызова...</p>
      ) : loadError ? (
        <div className="mt-6 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {loadError}
        </div>
      ) : (
        <div className="mt-6 max-w-xl rounded-lg border bg-card p-6 text-card-foreground shadow-sm">
          {challenge?.goal != null ? (
            <>
              <div className="flex items-center justify-between">
                <h2 className="text-xl font-semibold">{selectedYear} год</h2>
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
                  Цель на {selectedYear} год достигнута!
                </div>
              )}
            </>
          ) : (
            <>
              <h2 className="text-xl font-semibold">{selectedYear} год</h2>
              <p className="mt-3 text-sm text-muted-foreground">
                Цель на {selectedYear} год пока не задана. Вы уже прочитали {challenge?.booksRead ?? 0}{' '}
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
      )}

      {history.length > 0 && (
        <div className="mt-8">
          <h2 className="text-lg font-semibold">История</h2>
          <ul className="mt-3 max-w-xl divide-y rounded-lg border bg-card text-card-foreground shadow-sm">
            {history.map((item) => (
              <li key={item.year} className="flex items-center justify-between px-4 py-3">
                <button
                  type="button"
                  onClick={() => setSelectedYear(item.year)}
                  className="text-sm font-medium hover:underline"
                >
                  {item.year} год
                </button>
                <span className="text-sm text-muted-foreground">
                  {item.booksRead} из {item.goal} {item.goal != null ? pluralizeBooks(item.goal) : ''}
                  {item.progressPercent != null ? ` · ${item.progressPercent}%` : ''}
                  {item.completed ? ' · Цель достигнута' : ''}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </main>
  );
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
