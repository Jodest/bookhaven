'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AdminGenreStatus } from '@bookhaven/api-client';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { adminGenresApi } from '@/entities/genre';

export interface GenreStatusChange {
  ids: string[];
  status: AdminGenreStatus;
}

/**
 * Sets the status of one or more genres via POST /admin/genres/bulk-status
 * (single-row actions send a one-element list). On success it invalidates
 * the whole ['admin','genres'] prefix (catalog list + moderation queue) and
 * the dashboard counters. Callers pass per-call onSuccess/onError to mutate().
 */
export function useGenreStatusMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ids, status }: GenreStatusChange) => adminGenresApi.bulkStatus(ids, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
    },
  });
}
