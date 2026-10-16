'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { ScreenWorkInput } from '@bookhaven/api-client';
import { adminScreenWorksApi } from '@/entities/screen-work';

/**
 * Full-replace update of a work. Invalidates the work list and detail and
 * the owner sections (adaptations of every book, films of every author),
 * which show the work's title.
 */
export function useUpdateScreenWork(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: ScreenWorkInput) => adminScreenWorksApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'screen-works'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
    },
  });
}

/** Deletes a work without links (409 otherwise). */
export function useDeleteScreenWork(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => adminScreenWorksApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'screen-works'] });
    },
  });
}
