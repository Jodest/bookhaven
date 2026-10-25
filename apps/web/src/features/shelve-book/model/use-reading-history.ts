'use client';

import { readsApi, shelfKeys } from '@/entities/shelf';
import { useAuthStore } from '@/entities/session';
import type { UpdateReadInput } from '@bookhaven/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { invalidateShelfContents } from './invalidate-shelf-caches';

/**
 * The reader's own reading history of one book. The list loads only while the
 * disclosure is expanded; every successful change refreshes the shelves, the
 * read counter, the challenge, the home card and the adaptation vote rights.
 */
export function useReadingHistory(bookId: string, expanded: boolean) {
  const { user, isAuthenticated } = useAuthStore();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: shelfKeys.readsForBook(user?.id, bookId),
    queryFn: () => readsApi.list(bookId),
    enabled: isAuthenticated && expanded,
  });

  const onSuccess = () => invalidateShelfContents(queryClient);

  const create = useMutation({
    mutationFn: (input: { startedOn?: string | null; finishedOn: string }) =>
      readsApi.create({ bookId, ...input }),
    onSuccess,
  });
  const update = useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateReadInput }) =>
      readsApi.update(id, input),
    onSuccess,
  });
  const remove = useMutation({
    mutationFn: (id: string) => readsApi.remove(id),
    onSuccess,
  });

  return {
    reads: query.data ?? [],
    isLoading: query.isPending && query.fetchStatus !== 'idle',
    isError: query.isError,
    createRead: create.mutateAsync,
    updateRead: (id: string, input: UpdateReadInput) => update.mutateAsync({ id, input }),
    deleteRead: remove.mutateAsync,
    isSaving: create.isPending || update.isPending,
    isDeleting: remove.isPending,
  };
}
