'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAuthorsApi } from '@/entities/author';

/**
 * Enriches authors from Wikidata via POST /admin/authors/enrich-wikidata
 * (fill-empty-only; single-row actions send a one-element list). On success
 * it invalidates the ['admin','authors'] prefix (authors list + moderation
 * queue). Callers pass per-call onSuccess/onError to mutate().
 */
export function useEnrichAuthors() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => adminAuthorsApi.enrichWikidata(ids),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
    },
  });
}
