import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminCollectionsApi, COLLECTIONS_QUERY_KEY, collectionDetailKey } from '@/entities/curated-collection';

/** «Пересчитать сейчас»: replaces the cached detail with the new snapshot and refreshes the list. */
export function useRecomputeCollection(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => adminCollectionsApi.recompute(id),
    onSuccess: (detail) => {
      queryClient.setQueryData(collectionDetailKey(id), detail);
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY, exact: true });
    },
  });
}
