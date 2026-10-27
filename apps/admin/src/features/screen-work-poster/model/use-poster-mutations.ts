'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminScreenWorksApi } from '@/entities/screen-work';

/**
 * Upload or removal of a work's poster. Both refresh the work detail and
 * list (the poster is not part of the data form, so the form is untouched).
 */
export function usePosterMutations(id: string) {
  const queryClient = useQueryClient();
  const onSuccess = () => queryClient.invalidateQueries({ queryKey: ['admin', 'screen-works'] });
  const upload = useMutation({
    mutationFn: (file: File) => adminScreenWorksApi.uploadPoster(id, file),
    onSuccess,
  });
  const remove = useMutation({
    mutationFn: () => adminScreenWorksApi.deletePoster(id),
    onSuccess,
  });
  return { upload, remove, isPending: upload.isPending || remove.isPending };
}
