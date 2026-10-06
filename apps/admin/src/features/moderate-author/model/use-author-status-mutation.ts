'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AdminAuthorStatus } from '@bookhaven/api-client';
import { adminAuthorsApi } from '@/entities/author';

export interface AuthorStatusChange {
  ids: string[];
  status: AdminAuthorStatus;
}

/**
 * Sets the status of one or more authors via POST /admin/authors/bulk-status
 * (single-row actions send a one-element list). On success it invalidates
 * the whole ['admin','authors'] prefix (authors list + moderation queue) and
 * the dashboard counters. Callers pass per-call onSuccess/onError to mutate().
 */
export function useAuthorStatusMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ids, status }: AuthorStatusChange) => adminAuthorsApi.bulkStatus(ids, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
    },
  });
}
