'use client';

import { BookRow } from '@/entities/book';
import type { Route } from 'next';
import { usePersonalHome } from '../model/use-personal-home';
import { RowSkeleton } from './row-skeleton';

const CONTINUE_READING_TITLE = 'Продолжить чтение';

interface PersonalHomeProps {
  className?: string;
}

/**
 * "Продолжить чтение" row for logged-in users (see
 * docs/features/home-page-content.md). The challenge and want-to-read cards
 * are separate widgets (ChallengeProgress, WantToReadPick) sharing the same
 * queries. Renders nothing for guests and while the session is resolving (no
 * guest to user flash), a skeleton while loading, and hides an empty or
 * failed block.
 */
export function PersonalHome({ className }: PersonalHomeProps) {
  const { enabled, query } = usePersonalHome();

  if (!enabled) {
    return null;
  }

  const me = query.data;

  if (query.isPending) {
    return <RowSkeleton title={CONTINUE_READING_TITLE} className={className} />;
  }
  if (!me?.continueReading || me.continueReading.books.length === 0) {
    return null;
  }

  const { shelfId, total, books } = me.continueReading;
  return (
    <BookRow
      title={CONTINUE_READING_TITLE}
      books={books}
      href={`/shelves/${shelfId}` as Route}
      linkLabel={`Все (${total})`}
      className={className}
    />
  );
}
