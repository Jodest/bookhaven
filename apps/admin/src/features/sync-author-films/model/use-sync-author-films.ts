'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAuthorsApi } from '@/entities/author';

/**
 * Finds films about authors in Wikidata via
 * POST /admin/authors/sync-screen-works (single-row actions send a
 * one-element list). On success it invalidates the ['admin','authors'] prefix
 * (authors list, moderation queue and the films of every author). Callers
 * pass per-call onSuccess/onError to mutate().
 */
export function useSyncAuthorFilms() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => adminAuthorsApi.syncScreenWorks(ids),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
    },
  });
}
