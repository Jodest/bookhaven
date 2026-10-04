'use client';

import { BookRow } from '@/entities/book';
import { usePersonalHome } from '../model/use-personal-home';
import { RowSkeleton } from './row-skeleton';

const TITLE = 'Вам может понравиться';

interface PersonalRecommendationsProps {
  className?: string;
}

/**
 * "Вам может понравиться" row for logged-in users. Shares the GET /home/me
 * query with PersonalHome (one request). Renders nothing for guests, while
 * the session is resolving, on error and on an empty list.
 */
export function PersonalRecommendations({ className }: PersonalRecommendationsProps) {
  const { enabled, query } = usePersonalHome();

  if (!enabled) {
    return null;
  }
  if (query.isPending) {
    return <RowSkeleton title={TITLE} className={className} />;
  }
  if (!query.data || query.data.recommendations.length === 0) {
    return null;
  }
  return <BookRow title={TITLE} books={query.data.recommendations} className={className} />;
}
