import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminCollectionsApi, COLLECTIONS_QUERY_KEY } from '../api';

/** Bulk add of the chosen books; refreshes the admin collection queries. */
export function useBulkAddBooks(collectionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ bookIds, position }: { bookIds: string[]; position?: number }) =>
      adminCollectionsApi.bulkAddBooks(collectionId, bookIds, position),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY }),
  });
}
