'use client';

import type { ReadingChallenge } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useCurrentReadingChallenge } from '../model/use-personal-home';
import { CardSkeleton } from './row-skeleton';

interface ChallengeCardProps {
  year: number;
  challenge: ReadingChallenge;
}

/**
 * This year's reading-challenge card for the home sidebar (logged-in only):
 * skeleton while loading, nothing for guests, while the session resolves, or
 * on error.
 */
export function ChallengeProgress() {
  const { year, query } = useCurrentReadingChallenge();

  if (query.isPending) {
    // Disabled queries (guest / resolving session) stay pending: render nothing.
    return query.fetchStatus === 'idle' ? null : <CardSkeleton label="Читательский вызов" />;
  }
  if (!query.data) {
    return null;
  }
  return <ChallengeCard year={year} challenge={query.data} />;
}

/** Progress toward the goal (same bar markup as ManageReadingChallengeGoal), or a CTA to set a goal. */
function ChallengeCard({ year, challenge }: ChallengeCardProps) {
  const { goal, booksRead, progressPercent, completed } = challenge;

  return (
    <section
      aria-labelledby="home-challenge-heading"
      className="rounded-2xl border bg-card p-5 text-card-foreground shadow-sm"
    >
      <div className="flex items-baseline justify-between gap-4">
        <h2 id="home-challenge-heading" className="font-display text-xl font-semibold">
          Читательский вызов
        </h2>
        {goal != null && (
          <Link href="/challenge" className="shrink-0 text-sm font-medium text-primary hover:underline">
            Подробнее
          </Link>
        )}
      </div>

      {goal != null ? (
        <>
          <p className="mt-3 text-sm text-muted-foreground">
            Прочитано {booksRead} из {goal} в {year}
          </p>
          <div
            className="mt-2 h-3 w-full overflow-hidden rounded-full bg-muted"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progressPercent ?? 0}
            aria-label={`Прогресс читательского вызова ${year}`}
          >
            <div
              className="h-full rounded-full bg-primary transition-all"
              style={{ width: `${Math.min(progressPercent ?? 0, 100)}%` }}
            />
          </div>
          {completed && (
            <p className="mt-3 text-sm font-medium text-primary">Цель на {year} год достигнута!</p>
          )}
        </>
      ) : (
        <>
          <p className="mt-3 text-sm text-muted-foreground">
            Поставьте цель на {year} год и следите, сколько книг уже прочитано.
          </p>
          <Button className="mt-4" asChild>
            <Link href="/challenge">Поставить цель</Link>
          </Button>
        </>
      )}
    </section>
  );
}
