'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminAIDescriptionGenerateInput } from '@bookhaven/api-client';
import {
  adminAiDescriptionsApi,
  AI_DESCRIPTIONS_STATUS_QUERY_KEY,
  aiDescriptionQueryKey,
} from '@/entities/ai-description';

/** GET /admin/ai-descriptions/status (switch, model, counters). */
export function useAiDescriptionStatus() {
  return useQuery({
    queryKey: AI_DESCRIPTIONS_STATUS_QUERY_KEY,
    queryFn: () => adminAiDescriptionsApi.status(),
  });
}

/**
 * POST /admin/ai-descriptions/generate. Callers map errors with
 * generateErrorText. On success the status and the listed books' records
 * are refreshed.
 */
export function useGenerateAiDescription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: AdminAIDescriptionGenerateInput) => adminAiDescriptionsApi.generate(body),
    onSuccess: (_data, body) => {
      queryClient.invalidateQueries({ queryKey: AI_DESCRIPTIONS_STATUS_QUERY_KEY });
      if ('book_ids' in body) {
        body.book_ids.forEach((id) => queryClient.invalidateQueries({ queryKey: aiDescriptionQueryKey(id) }));
      }
    },
  });
}
