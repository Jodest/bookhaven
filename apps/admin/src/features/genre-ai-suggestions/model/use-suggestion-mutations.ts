'use client';

import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';
import type { AdminGenreAIApplyItem } from '@bookhaven/api-client';
import { COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { adminGenreAiSuggestionsApi, GENRE_AI_SUGGESTIONS_QUERY_KEY } from '@/entities/genre-ai-suggestion';

// Applying changes genres (merge / reject), so the suggestions, the status
// (open count), the genre queue and lists, books, the dashboard counters and
// the collections are all refreshed.
function invalidateAfterApply(queryClient: QueryClient) {
  queryClient.invalidateQueries({ queryKey: GENRE_AI_SUGGESTIONS_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
  queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
  queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
  queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
}

/** POST /admin/genres/ai-suggestions/:id/apply; targetId overrides the suggested merge target. */
export function useApplySuggestion() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, targetId }: { id: string; targetId?: string }) =>
      adminGenreAiSuggestionsApi.apply(id, targetId),
    // A stale / not found answer also changed the suggestion's status.
    onSettled: () => invalidateAfterApply(queryClient),
  });
}

/** POST /admin/genres/ai-suggestions/apply with the suggested targets. */
export function useBulkApplySuggestions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (items: AdminGenreAIApplyItem[]) => adminGenreAiSuggestionsApi.bulkApply(items),
    onSettled: () => invalidateAfterApply(queryClient),
  });
}

/** POST /admin/genres/ai-suggestions/:id/dismiss. Changes no genre. */
export function useDismissSuggestion() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminGenreAiSuggestionsApi.dismiss(id),
    onSettled: () => queryClient.invalidateQueries({ queryKey: GENRE_AI_SUGGESTIONS_QUERY_KEY }),
  });
}

/** POST /admin/genres/ai-suggestions/dismiss. */
export function useBulkDismissSuggestions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => adminGenreAiSuggestionsApi.bulkDismiss(ids),
    onSettled: () => queryClient.invalidateQueries({ queryKey: GENRE_AI_SUGGESTIONS_QUERY_KEY }),
  });
}

/** POST /admin/genres/ai-suggestions/analyze. Only queues a background run. */
export function useAnalyzeGenres() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => adminGenreAiSuggestionsApi.analyze(),
    onSettled: () => queryClient.invalidateQueries({ queryKey: GENRE_AI_SUGGESTIONS_QUERY_KEY }),
  });
}
