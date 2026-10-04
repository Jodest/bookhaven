import type { ReadingChallenge } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';

interface ChallengeProgressProps {
  year: number;
  challenge: ReadingChallenge;
}

/**
 * This year's reading-challenge card: progress toward the goal (same bar
 * markup as ManageReadingChallengeGoal), or a CTA to set a goal.
 */
export function ChallengeProgress({ year, challenge }: ChallengeProgressProps) {
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
