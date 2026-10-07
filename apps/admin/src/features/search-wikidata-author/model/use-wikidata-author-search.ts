'use client';

import { useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AdminHarvestAuthorCandidate, ApiError } from '@bookhaven/api-client';
import { adminHarvestApi } from '@/entities/harvest';

export interface WikidataAuthorSearch {
  name: string;
  setName: (name: string) => void;
  candidates: AdminHarvestAuthorCandidate[];
  error: string | null;
  hasSearched: boolean;
  isPending: boolean;
  /** Searches for the current input value (no-op when blank). */
  search: () => void;
  /** Re-runs the last successful-or-failed search query, e.g. to refresh `existing` markers. */
  refresh: () => void;
}

/**
 * State and search mutation of the Wikidata author search widget
 * (GET /admin/harvest/authors/search). It lives in a hook so the parent can
 * re-run the last search (`refresh`) after an action changed the catalog.
 */
export function useWikidataAuthorSearch(): WikidataAuthorSearch {
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [candidates, setCandidates] = useState<AdminHarvestAuthorCandidate[]>([]);
  const [hasSearched, setHasSearched] = useState(false);
  const lastQuery = useRef<string | null>(null);

  const mutation = useMutation({
    mutationFn: adminHarvestApi.searchAuthors,
    onSuccess: (data) => {
      setError(null);
      setCandidates(data.items);
      setHasSearched(true);
    },
    onError: (err) => {
      let message = 'Не удалось выполнить поиск. Попробуйте снова.';
      if (err instanceof ApiError && err.message) {
        message = err.message;
      }
      setError(message);
      setCandidates([]);
      setHasSearched(true);
    },
  });

  const search = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    lastQuery.current = trimmed;
    mutation.mutate(trimmed);
  };

  const refresh = () => {
    if (lastQuery.current) {
      mutation.mutate(lastQuery.current);
    }
  };

  return {
    name,
    setName,
    candidates,
    error,
    hasSearched,
    isPending: mutation.isPending,
    search,
    refresh,
  };
}
