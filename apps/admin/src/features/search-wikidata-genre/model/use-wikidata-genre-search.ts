'use client';

import { useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AdminGenreWikidataCandidate, ApiError } from '@bookhaven/api-client';
import { adminGenresApi } from '@/entities/genre';

export interface WikidataGenreSearch {
  name: string;
  setName: (name: string) => void;
  candidates: AdminGenreWikidataCandidate[];
  /** 'unavailable' when the server could not check the candidates are literary genres. */
  classCheck: 'ok' | 'unavailable';
  error: string | null;
  hasSearched: boolean;
  isPending: boolean;
  /** Searches for the current input value (no-op when blank). */
  search: () => void;
  /** Re-runs the last query, e.g. to refresh `existing` markers after an import. */
  refresh: () => void;
}

/**
 * State and search mutation of the Wikidata genre search widget
 * (GET /admin/genres/wikidata-search, docs/features/genre-wikidata-import.md).
 * It lives in a hook so the parent can re-run the last search (`refresh`).
 */
export function useWikidataGenreSearch(): WikidataGenreSearch {
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [candidates, setCandidates] = useState<AdminGenreWikidataCandidate[]>([]);
  const [classCheck, setClassCheck] = useState<'ok' | 'unavailable'>('ok');
  const [hasSearched, setHasSearched] = useState(false);
  const lastQuery = useRef<string | null>(null);

  const mutation = useMutation({
    mutationFn: adminGenresApi.searchWikidata,
    onSuccess: (data) => {
      setError(null);
      setCandidates(data.items);
      setClassCheck(data.class_check);
      setHasSearched(true);
    },
    onError: (err) => {
      let message = 'Не удалось выполнить поиск. Попробуйте снова.';
      if (err instanceof ApiError && err.message) {
        message = err.message;
      }
      setError(message);
      setCandidates([]);
      setClassCheck('ok');
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
    classCheck,
    error,
    hasSearched,
    isPending: mutation.isPending,
    search,
    refresh,
  };
}
