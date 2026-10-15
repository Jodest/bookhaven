import { useQuery } from '@tanstack/react-query';
import { collectionsApi } from '../api';
import { COLLECTION_PAGE_SIZE } from '../lib/place-href';

export const CURATED_COLLECTIONS_KEY = ['curated-collections'] as const;

export { COLLECTION_PAGE_SIZE };

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
