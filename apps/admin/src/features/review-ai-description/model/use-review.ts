'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAiDescriptionsApi, AI_DESCRIPTIONS_QUERY_KEY } from '@/entities/ai-description';

/**
 * POST /admin/ai-descriptions/:book_id/approve (optional edited text).
 * Errors map with reviewErrorText. Every AI descriptions query (status,
 * lists, records) refreshes on success.
 */
export function useApproveAiDescription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ bookId, text }: { bookId: string; text?: string }) =>
      adminAiDescriptionsApi.approve(bookId, text),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: AI_DESCRIPTIONS_QUERY_KEY }),
  });
}

/** POST /admin/ai-descriptions/:book_id/discard. */
export function useDiscardAiDescription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (bookId: string) => adminAiDescriptionsApi.discard(bookId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: AI_DESCRIPTIONS_QUERY_KEY }),
  });
}
