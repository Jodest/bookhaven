'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { CatalogListFilter, CatalogListState, CatalogStatus } from './types';

function parseStatus(value: string | null): CatalogStatus | undefined {
  return value === 'pending' || value === 'published' || value === 'rejected' ? value : undefined;
}

/**
 * Catalog list state from the URL (status/search/page + configured filters,
 * passed through raw: async filter options are not validated here) and a
 * pushParams helper that drops empty keys and pushes the new URL.
 */
export function useCatalogUrlState(filters: CatalogListFilter[], limit: number) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const search = (searchParams.get('search') || '').trim();
  const filterValues: Record<string, string | undefined> = {};
  filters.forEach((filter) => {
    filterValues[filter.param] = searchParams.get(filter.param) || undefined;
  });

  const state: CatalogListState = {
    status: parseStatus(searchParams.get('status')),
    search: search || undefined,
    page: Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1),
    limit,
    filters: filterValues,
  };

  const pushParams = (updates: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });
    const qs = next.toString();
    router.push((qs ? `${pathname}?${qs}` : pathname) as any);
  };

  return { state, search, pathname, searchParams, pushParams };
}
