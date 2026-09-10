'use client';

import { readingChallengesApi } from '@/entities/reading-challenge';
import { useAuthStore } from '@/entities/session';
import { ManageReadingChallengeGoal } from '@/features/manage-reading-challenge';
import { ReadingChallenge } from '@bookhaven/api-client';
import { useEffect, useMemo, useState } from 'react';

const currentYear = new Date().getUTCFullYear();

function pluralizeBooks(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'книга';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'книги';
  return 'книг';
}

export function ChallengeView() {
  const { user, isAuthenticated, isLoading: authLoading } = useAuthStore();

  const [selectedYear, setSelectedYear] = useState(currentYear);
  const [challenge, setChallenge] = useState<ReadingChallenge | null>(null);
  const [history, setHistory] = useState<ReadingChallenge[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const fetchHistory = async () => {
    const response = await readingChallengesApi.list();
    setHistory(response.items);
  };

  const fetchYear = async (year: number) => {
    const data = await readingChallengesApi.get(year);
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

  const availableYears = useMemo(() => {
    const years = new Set<number>();
    years.add(currentYear);
    years.add(selectedYear);
    for (const item of history) {
      years.add(item.year);
    }
    return Array.from(years).sort((a, b) => b - a);
  }, [history, selectedYear]);

  const handleSaved = (updated: ReadingChallenge) => {
    setChallenge(updated);
    fetchHistory();
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
        <ManageReadingChallengeGoal year={selectedYear} challenge={challenge} onSaved={handleSaved} />
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
