'use client';

import { BookRow } from '@/entities/book';
import type { Route } from 'next';
import type { ReactNode } from 'react';
import { useCurrentReadingChallenge, usePersonalHome } from '../model/use-personal-home';
import { ChallengeProgress } from './challenge-progress';
import { CardSkeleton, RowSkeleton } from './row-skeleton';
import { WantToReadPick } from './want-to-read-pick';

const CONTINUE_READING_TITLE = 'Продолжить чтение';

interface PersonalHomeProps {
  className?: string;
}

/**
 * Personal section of the home page for logged-in users (see
 * docs/features/home-page-content.md): "Продолжить чтение", then the
 * reading-challenge card and the want-to-read pick side by side at md+.
 * Renders nothing for guests and while the session is resolving (no guest
 * to user flash), skeletons while loading, and hides empty or failed blocks.
 */
export function PersonalHome({ className }: PersonalHomeProps) {
  const { enabled, query } = usePersonalHome();
  const { year, query: challengeQuery } = useCurrentReadingChallenge();

  if (!enabled) {
    return null;
  }

  const me = query.data;

  let continueReading: ReactNode = null;
  if (query.isPending) {
    continueReading = <RowSkeleton title={CONTINUE_READING_TITLE} />;
  } else if (me?.continueReading && me.continueReading.books.length > 0) {
    const { shelfId, total, books } = me.continueReading;
    continueReading = (
      <BookRow
        title={CONTINUE_READING_TITLE}
        books={books}
        href={`/shelves/${shelfId}` as Route}
        linkLabel={`Все (${total})`}
      />
    );
  }

  const cards: ReactNode[] = [];
  if (challengeQuery.isPending) {
    cards.push(<CardSkeleton key="challenge" label="Читательский вызов" />);
  } else if (challengeQuery.data) {
    cards.push(<ChallengeProgress key="challenge" year={year} challenge={challengeQuery.data} />);
  }
  if (query.isPending) {
    cards.push(<CardSkeleton key="want-to-read" label="Из вашего «Хочу прочитать»" />);
  } else if (me?.wantToReadPick?.book) {
    const { shelfId, total, book } = me.wantToReadPick;
    cards.push(<WantToReadPick key="want-to-read" shelfId={shelfId} total={total} book={book} />);
  }

  if (!continueReading && cards.length === 0) {
    return null;
  }

  return (
    <div
      className={`container mx-auto space-y-8 px-4 pb-12 sm:px-6 lg:px-8 ${className ?? ''}`}
      data-testid="personal-home"
    >
      {continueReading}
      {cards.length > 0 && (
        <div className={cards.length > 1 ? 'grid grid-cols-1 gap-6 md:grid-cols-2' : 'grid grid-cols-1 gap-6'}>
          {cards}
        </div>
      )}
    </div>
  );
}
