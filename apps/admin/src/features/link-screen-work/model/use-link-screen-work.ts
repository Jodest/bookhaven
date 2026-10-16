'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AddScreenWorkLinkBody } from '@bookhaven/api-client';
import { adminAuthorsApi } from '@/entities/author';
import { adminBooksApi } from '@/entities/book';

export interface LinkTarget {
  type: 'book' | 'author';
  id: string;
  /** Ids of the works already linked (hidden ones included). */
  linkedIds: string[];
}

export interface LinkInput {
  body: AddScreenWorkLinkBody;
  /** Authors only. */
  relation: 'biopic' | 'documentary';
}

/**
 * Manual link of an existing or a new work to a book (adaptation) or an
 * author (film). Invalidates the owner's list and the work list.
 */
export function useLinkScreenWork(target: Pick<LinkTarget, 'type' | 'id'>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ body, relation }: LinkInput) =>
      target.type === 'book'
        ? adminBooksApi.addAdaptation(target.id, body)
        : adminAuthorsApi.addScreenWork(target.id, { ...body, relation }),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey:
          target.type === 'book'
            ? ['admin', 'books', target.id, 'adaptations']
            : ['admin', 'authors', target.id, 'screen-works'],
      });
      queryClient.invalidateQueries({ queryKey: ['admin', 'screen-works'] });
    },
  });
}
