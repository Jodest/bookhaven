import { useMutation, useQueryClient } from '@tanstack/react-query';
import { adminCollectionsApi, COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';

/** Add / remove / move mutations of one collection; each refreshes the admin collection queries. */
export function useCollectionBookMutations(collectionId: string) {
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });

  const add = useMutation({
    mutationFn: ({ bookId, position }: { bookId: string; position?: number }) =>
      adminCollectionsApi.addBook(collectionId, bookId, position),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (bookId: string) => adminCollectionsApi.removeBook(collectionId, bookId),
    onSuccess: refresh,
  });
  const move = useMutation({
    mutationFn: ({ bookId, position }: { bookId: string; position: number }) =>
      adminCollectionsApi.moveBook(collectionId, bookId, position),
    onSuccess: refresh,
  });

  return { add, remove, move };
}
