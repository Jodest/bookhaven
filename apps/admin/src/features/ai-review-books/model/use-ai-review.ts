'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminBookAIReviewRequest } from '@bookhaven/api-client';
import { adminBooksApi } from '@/entities/book';

export const AI_REVIEW_STATUS_QUERY_KEY = ['admin', 'books', 'ai-review', 'status'] as const;

/** GET /admin/books/ai-review/status (enabled, model, needs_review). */
export function useAiReviewStatus() {
  return useQuery({
    queryKey: AI_REVIEW_STATUS_QUERY_KEY,
    queryFn: () => adminBooksApi.aiReviewStatus(),
    refetchOnWindowFocus: true,
  });
}

/**
 * POST /admin/books/ai-review. Only queues a background check; verdicts
 * show up later. Callers map errors with aiReviewErrorText (409, 503).
 */
export function useAiReviewMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: AdminBookAIReviewRequest) => adminBooksApi.aiReview(body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AI_REVIEW_STATUS_QUERY_KEY });
    },
  });
}
