'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminBooksApi } from '@/entities/book';

/**
 * Finds screen adaptations of books in Wikidata via
 * POST /admin/books/sync-adaptations (single-row actions send a one-element
 * list). On success it invalidates the ['admin','books'] prefix (books list,
 * moderation queue and the adaptations of every book). Callers pass per-call
 * onSuccess/onError to mutate().
 */
export function useSyncAdaptations() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => adminBooksApi.syncAdaptations(ids),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
    },
  });
}
