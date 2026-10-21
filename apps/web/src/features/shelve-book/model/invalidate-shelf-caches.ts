import { homeKeys } from '@/entities/home';
import { readingChallengeKeys } from '@/entities/reading-challenge';
import { screenWorkKeys } from '@/entities/screen-work';
import { shelfKeys } from '@/entities/shelf';
import type { QueryClient } from '@tanstack/react-query';

/**
 * Call after a change of which books are on the reader's shelves: marks the
 * shelves, the reading challenge, the personal home payload and the adaptation
 * verdicts (can_vote depends on «Прочитано») as stale.
 */
export function invalidateShelfContents(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: shelfKeys.all }),
    queryClient.invalidateQueries({ queryKey: readingChallengeKeys.all }),
    queryClient.invalidateQueries({ queryKey: homeKeys.all }),
    queryClient.invalidateQueries({ queryKey: screenWorkKeys.all }),
  ]);
}

/** Call after shelf edits that do not change shelf contents (create, rename, privacy). */
export function invalidateShelfMeta(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: shelfKeys.all });
}
