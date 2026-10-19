import { homeKeys } from '@/entities/home';
import { readingChallengeKeys } from '@/entities/reading-challenge';
import { shelfKeys } from '@/entities/shelf';
import type { QueryClient } from '@tanstack/react-query';

/**
 * Call after a change of which books are on the reader's shelves: marks the
 * shelves, the reading challenge and the personal home payload as stale.
 */
export function invalidateShelfContents(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: shelfKeys.all }),
    queryClient.invalidateQueries({ queryKey: readingChallengeKeys.all }),
    queryClient.invalidateQueries({ queryKey: homeKeys.all }),
  ]);
}

/** Call after shelf edits that do not change shelf contents (create, rename, privacy). */
export function invalidateShelfMeta(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: shelfKeys.all });
}
