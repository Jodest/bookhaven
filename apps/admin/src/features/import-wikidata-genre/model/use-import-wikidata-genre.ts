'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AdminGenreWikidataCandidate } from '@bookhaven/api-client';
import { adminGenresApi } from '@/entities/genre';

/**
 * Imports a Wikidata genre (POST /admin/genres/import-wikidata,
 * docs/features/genre-wikidata-import.md). An import error rejects the
 * returned promise. Every successful import invalidates the admin genre
 * queries (['admin','genres'] prefix: list and moderation queue) and the
 * dashboard counters.
 */
export function useImportWikidataGenre() {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: (candidate: AdminGenreWikidataCandidate) => adminGenresApi.importWikidata(candidate.qid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
    },
  });

  return {
    importGenre: (candidate: AdminGenreWikidataCandidate) => mutation.mutateAsync(candidate),
    /** QID of the candidate being imported, or null. */
    pendingQid: mutation.isPending ? (mutation.variables?.qid ?? null) : null,
  };
}
