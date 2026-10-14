import { useQuery } from '@tanstack/react-query';
import { collectionsApi } from '../api';

export const CURATED_COLLECTIONS_KEY = ['curated-collections'] as const;

/** Page size of /top/{slug}; the API default and the value the page numbers are based on. */
export const COLLECTION_PAGE_SIZE = 50;

export function useCuratedCollections() {
  return useQuery({
    queryKey: CURATED_COLLECTIONS_KEY,
    queryFn: () => collectionsApi.list(),
  });
}

export function useCuratedCollection(slug: string, page: number) {
  return useQuery({
    queryKey: [...CURATED_COLLECTIONS_KEY, slug, page],
    queryFn: () => collectionsApi.get(slug, { page, limit: COLLECTION_PAGE_SIZE }),
    retry: false,
  });
}
