'use client';

import { shelfKeys, shelvesApi } from '@/entities/shelf';
import { useAuthStore } from '@/entities/session';
import type { Shelf } from '@bookhaven/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { invalidateShelfContents } from './invalidate-shelf-caches';

export type DefaultShelfType = 'want_to_read' | 'reading' | 'read';

export const defaultTypeOrder: DefaultShelfType[] = ['want_to_read', 'reading', 'read'];
export const defaultTypeLabels: Record<DefaultShelfType, string> = {
  want_to_read: 'Хочу прочитать',
  reading: 'Читаю',
  read: 'Прочитано',
};

// Shelf.type is a wider union (custom shelves); compare as plain strings.
export const isDefaultType = (type: string) => (defaultTypeOrder as string[]).includes(type);

export type ShelvesStatus = 'loading' | 'ready' | 'error';

const TOGGLE_MUTATION_KEY = [...shelfKeys.all, 'toggle-book'] as const;

interface ToggleVariables {
  shelf: Shelf;
  wasActive: boolean;
  kind: 'default' | 'custom';
}

/**
 * The reader's shelves for one book, with optimistic toggles. Default
 * (system) shelves are mutually exclusive; custom shelves are independent.
 */
export function useBookShelves(bookId: string) {
  const { user, isAuthenticated } = useAuthStore();
  const queryClient = useQueryClient();
  const queryKey = shelfKeys.forBook(user?.id, bookId);
  const [toggleError, setToggleError] = useState<string | null>(null);

  const query = useQuery({
    queryKey,
    queryFn: () => shelvesApi.list({ book_id: bookId }).then((response) => response.items),
    enabled: isAuthenticated,
  });

  const mutation = useMutation({
    mutationKey: TOGGLE_MUTATION_KEY,
    mutationFn: ({ shelf, wasActive }: ToggleVariables) =>
      wasActive ? shelvesApi.removeBook(shelf.id, bookId) : shelvesApi.addBook(shelf.id, bookId),
    onMutate: async ({ shelf, wasActive, kind }) => {
      setToggleError(null);
      await queryClient.cancelQueries({ queryKey });
      queryClient.setQueryData<Shelf[]>(queryKey, (prev) =>
        prev?.map((s) => {
          if (kind === 'default') {
            return isDefaultType(s.type)
              ? { ...s, hasBook: s.type === shelf.type ? !wasActive : false }
              : s;
          }
          return s.id === shelf.id ? { ...s, hasBook: !wasActive } : s;
        })
      );
    },
    onError: (err, { shelf, wasActive, kind }) => {
      queryClient.setQueryData<Shelf[]>(queryKey, (prev) =>
        prev?.map((s) => {
          if (kind === 'default') {
            return isDefaultType(s.type) && s.type === shelf.type ? { ...s, hasBook: wasActive } : s;
          }
          return s.id === shelf.id ? { ...s, hasBook: wasActive } : s;
        })
      );
      setToggleError(err instanceof Error ? err.message : 'Ошибка обновления полки');
    },
    onSettled: () => {
      // Only the last in-flight toggle refetches, so a refetch never overwrites
      // the optimistic state of a toggle that is still running.
      if (queryClient.isMutating({ mutationKey: TOGGLE_MUTATION_KEY }) === 1) {
        void invalidateShelfContents(queryClient);
      }
    },
  });

  const { mutateAsync } = mutation;

  const shelves = useMemo(() => query.data ?? [], [query.data]);
  const status: ShelvesStatus = !isAuthenticated
    ? 'ready'
    : query.isPending
      ? 'loading'
      : query.isError
        ? 'error'
        : 'ready';
  const queryError = query.isError
    ? query.error instanceof Error
      ? query.error.message
      : 'Ошибка загрузки полок'
    : null;
  const error = toggleError ?? queryError;

  const toggleDefault = useCallback(
    async (shelf: Shelf) => {
      if (!isAuthenticated) return;
      await mutateAsync({ shelf, wasActive: Boolean(shelf.hasBook), kind: 'default' }).catch(() => {});
    },
    [isAuthenticated, mutateAsync]
  );

  const toggleCustom = useCallback(
    async (shelf: Shelf) => {
      if (!isAuthenticated) return;
      await mutateAsync({ shelf, wasActive: Boolean(shelf.hasBook), kind: 'custom' }).catch(() => {});
    },
    [isAuthenticated, mutateAsync]
  );

  /** Removes the book from whichever default shelf currently holds it. */
  const clearStatus = useCallback(async () => {
    const current = shelves.find((s) => isDefaultType(s.type) && s.hasBook);
    if (current) await toggleDefault(current);
  }, [shelves, toggleDefault]);

  return { isAuthenticated, shelves, status, error, toggleDefault, toggleCustom, clearStatus };
}
