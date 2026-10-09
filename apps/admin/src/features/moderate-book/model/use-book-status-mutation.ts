'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AdminBookStatus } from '@bookhaven/api-client';
import { adminBooksApi } from '@/entities/book';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';

export interface BookStatusChange {
  ids: string[];
  status: AdminBookStatus;
}

/**
 * Sets the status of one or more books via POST /admin/books/bulk-status
 * (single-row actions send a one-element list). On success it invalidates
 * the whole ['admin','books'] prefix (catalog list + moderation queue) and
 * the dashboard counters. Callers pass per-call onSuccess/onError to mutate().
 * Shared by the catalog (unpublish) and the book moderation queue (F4).
 */
export function useBookStatusMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ids, status }: BookStatusChange) => adminBooksApi.bulkStatus(ids, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
    },
  });
}
