import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AdminCuratedCollection, AdminCuratedCollectionInput } from '@bookhaven/api-client';
import { adminCollectionsApi, COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';

export function useCreateCollection(onCreated: (created: AdminCuratedCollection) => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: AdminCuratedCollectionInput) => adminCollectionsApi.create(data),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
      onCreated(created);
    },
  });
}

export function useUpdateCollection(id: string, onUpdated?: (updated: AdminCuratedCollection) => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: AdminCuratedCollectionInput) => adminCollectionsApi.update(id, data),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
      onUpdated?.(updated);
    },
  });
}

export function useDeleteCollection(id: string, onDeleted: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => adminCollectionsApi.remove(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
      onDeleted();
    },
  });
}
