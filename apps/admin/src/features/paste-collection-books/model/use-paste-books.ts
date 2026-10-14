import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminCollectionsApi, COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';

/** Read-only preview of a pasted list. */
export function usePastePreview(collectionId: string) {
  return useMutation({
    mutationFn: (text: string) => adminCollectionsApi.previewPaste(collectionId, text),
  });
}

/** Bulk add of the chosen books; refreshes the admin collection queries. */
export function useBulkAdd(collectionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ bookIds, position }: { bookIds: string[]; position?: number }) =>
      adminCollectionsApi.bulkAddBooks(collectionId, bookIds, position),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY }),
  });
}
