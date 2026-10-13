'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminCurationCreateInput } from '@bookhaven/api-client';
import {
  adminAiCuratorApi,
  CURATION_REQUESTS_QUERY_KEY,
  CURATION_STATUS_QUERY_KEY,
  curationRequestQueryKey,
  isCurationActive,
} from '@/entities/ai-curation';

/** Poll interval of a queued/running request. */
export const CURATION_POLL_MS = 3000;

/** GET /admin/ai-curator/status (switch, limits, the caller's usage). */
export function useCurationStatus() {
  return useQuery({
    queryKey: CURATION_STATUS_QUERY_KEY,
    queryFn: () => adminAiCuratorApi.status(),
    refetchOnWindowFocus: true,
  });
}

/** GET /admin/ai-curator/requests (newest 20 of all admins). */
export function useCurationRequests() {
  return useQuery({
    queryKey: CURATION_REQUESTS_QUERY_KEY,
    queryFn: () => adminAiCuratorApi.list(),
  });
}

/**
 * GET /admin/ai-curator/requests/:id, polled every 3 s while the request is
 * queued or running. When it finishes, the status and the list refresh.
 */
export function useCurationRequest(id: string | null) {
  const queryClient = useQueryClient();
  return useQuery({
    queryKey: curationRequestQueryKey(id ?? ''),
    queryFn: async () => {
      const req = await adminAiCuratorApi.get(id as string);
      if (!isCurationActive(req.status)) {
        // exact: the list key is a prefix of this detail key; a prefix
        // invalidation would refetch this query in a loop.
        queryClient.invalidateQueries({ queryKey: CURATION_STATUS_QUERY_KEY, exact: true });
        queryClient.invalidateQueries({ queryKey: CURATION_REQUESTS_QUERY_KEY, exact: true });
      }
      return req;
    },
    enabled: !!id,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status && isCurationActive(status) ? CURATION_POLL_MS : false;
    },
  });
}

/**
 * POST /admin/ai-curator/requests. Resolves with the new request id;
 * callers map errors with createCurationErrorText. Status and the list are
 * refreshed after success and after a 409 (the active request is shown).
 */
export function useCreateCuration() {
  const queryClient = useQueryClient();
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: CURATION_STATUS_QUERY_KEY });
    queryClient.invalidateQueries({ queryKey: CURATION_REQUESTS_QUERY_KEY });
  };
  const mutation = useMutation({
    mutationFn: async (body: AdminCurationCreateInput) => {
      const created = await adminAiCuratorApi.create(body);
      return created.id;
    },
    onSuccess: refresh,
    onError: refresh,
  });
  return {
    create: (body: AdminCurationCreateInput) => mutation.mutateAsync(body),
    isPending: mutation.isPending,
    error: mutation.error,
    reset: mutation.reset,
  };
}
