'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AdminAuthorImportResult, AdminHarvestAuthorCandidate, ApiError } from '@bookhaven/api-client';
import { adminAuthorsApi } from '@/entities/author';
import { adminHarvestApi, HARVEST_JOBS_QUERY_KEY } from '@/entities/harvest';

export type HarvestStep =
  | { state: 'not_requested' }
  | { state: 'skipped' }
  | { state: 'started' }
  | { state: 'failed'; error: string };

export interface WikidataImportOutcome {
  result: AdminAuthorImportResult;
  withBooks: boolean;
  harvest: HarvestStep;
}

interface ImportVariables {
  candidate: AdminHarvestAuthorCandidate;
  withBooks: boolean;
}

function harvestErrorText(err: unknown): string {
  if (err instanceof ApiError && err.message) return err.message;
  if (err instanceof Error && err.message) return err.message;
  return 'неизвестная ошибка';
}

/**
 * Imports a Wikidata author (POST /admin/authors/import-wikidata) and, for
 * the "with books" action, then starts the unchanged harvest
 * (POST /admin/harvest) for the stored QID - only when the import outcome
 * is created or existing (docs/features/author-direct-import.md rule 4).
 *
 * An import error rejects the returned promise; a harvest error after a
 * successful import resolves with harvest.state = 'failed' (the author is
 * already imported). Invalidates ['admin','authors'] and the dashboard after
 * any successful import, and the harvest jobs list after a started harvest.
 */
export function useImportWikidataAuthor() {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async ({ candidate, withBooks }: ImportVariables): Promise<WikidataImportOutcome> => {
      const result = await adminAuthorsApi.importWikidata(candidate.qid);
      if (!withBooks) {
        return { result, withBooks, harvest: { state: 'not_requested' } };
      }
      const qid = result.author.external_id;
      if (result.outcome === 'rejected' || !qid) {
        return { result, withBooks, harvest: { state: 'skipped' } };
      }
      try {
        await adminHarvestApi.trigger({ author_qids: [qid] });
        return { result, withBooks, harvest: { state: 'started' } };
      } catch (err) {
        return { result, withBooks, harvest: { state: 'failed', error: harvestErrorText(err) } };
      }
    },
    onSuccess: (outcome) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
      if (outcome.harvest.state === 'started') {
        queryClient.invalidateQueries({ queryKey: HARVEST_JOBS_QUERY_KEY });
      }
    },
  });

  return {
    importAuthor: (candidate: AdminHarvestAuthorCandidate) =>
      mutation.mutateAsync({ candidate, withBooks: false }),
    importWithBooks: (candidate: AdminHarvestAuthorCandidate) =>
      mutation.mutateAsync({ candidate, withBooks: true }),
    /** QID of the candidate being imported, or null. */
    pendingQid: mutation.isPending ? (mutation.variables?.candidate.qid ?? null) : null,
  };
}
