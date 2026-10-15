'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminAiCollectionDraftCreateInput, AiDraftStatus } from '@bookhaven/api-client';
import {
  adminAiCollectionsApi,
  AI_COLLECTIONS_STATUS_QUERY_KEY,
  aiDraftQueryKey,
  aiDraftsQueryKey,
} from '../api';

/** Poll interval of a queued/running draft. */
export const AI_DRAFT_POLL_MS = 3000;

export function isDraftActive(status: AiDraftStatus): boolean {
  return status === 'queued' || status === 'running';
}

/** GET /admin/ai-collections/status (switch, limits, the caller's usage). */
export function useAiCollectionsStatus() {
  return useQuery({
    queryKey: AI_COLLECTIONS_STATUS_QUERY_KEY,
    queryFn: () => adminAiCollectionsApi.status(),
    refetchOnWindowFocus: true,
  });
}

/** GET /admin/ai-collections/drafts?collection_id= (newest 20, works while the feature is off). */
export function useAiDrafts(collectionId: string) {
  return useQuery({
    queryKey: aiDraftsQueryKey(collectionId),
    queryFn: () => adminAiCollectionsApi.list(collectionId),
  });
}

/**
 * GET /admin/ai-collections/drafts/:id, refetched every 3 s only while the
 * draft is queued or running (the server returns report: null then, so the
 * polling never runs the catalog matching). A finished draft keeps its
 * report (staleTime Infinity) until it is invalidated, e.g. after an apply.
 * When a draft finishes, the status and the drafts list refresh.
 */
export function useAiDraft(id: string | null, collectionId: string) {
  const queryClient = useQueryClient();
  return useQuery({
    queryKey: aiDraftQueryKey(id ?? ''),
    queryFn: async () => {
      const draft = await adminAiCollectionsApi.get(id as string);
      if (!isDraftActive(draft.status)) {
        queryClient.invalidateQueries({ queryKey: AI_COLLECTIONS_STATUS_QUERY_KEY, exact: true });
        queryClient.invalidateQueries({ queryKey: aiDraftsQueryKey(collectionId), exact: true });
      }
      return draft;
    },
    enabled: !!id,
    staleTime: Infinity,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status && isDraftActive(status) ? AI_DRAFT_POLL_MS : false;
    },
  });
}

/**
 * POST /admin/ai-collections/drafts. Resolves with the new draft id; callers
 * map errors with createDraftErrorText. The status and the drafts list are
 * refreshed after success and after a 409.
 */
export function useCreateAiDraft(collectionId: string) {
  const queryClient = useQueryClient();
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: AI_COLLECTIONS_STATUS_QUERY_KEY });
    queryClient.invalidateQueries({ queryKey: aiDraftsQueryKey(collectionId) });
  };
  const mutation = useMutation({
    mutationFn: async (body: Omit<AdminAiCollectionDraftCreateInput, 'collection_id'>) => {
      const created = await adminAiCollectionsApi.create({ ...body, collection_id: collectionId });
      return created.id;
    },
    onSuccess: refresh,
    onError: refresh,
  });
  return {
    create: (body: Omit<AdminAiCollectionDraftCreateInput, 'collection_id'>) => mutation.mutateAsync(body),
    isPending: mutation.isPending,
    error: mutation.error,
    reset: mutation.reset,
  };
}
