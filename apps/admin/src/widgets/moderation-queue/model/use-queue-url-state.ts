'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { ModerationQueueFilter, ModerationQueueState, ModerationStatus } from './types';

function parseStatus(value: string | null): ModerationStatus {
  return value === 'published' || value === 'rejected' ? value : 'pending';
}

/**
 * Queue state from the URL (status/search/page + configured filters, each
 * filter value kept only if it is one of its non-empty options) and an
 * updateParams helper that drops empty keys and pushes the new URL.
 */
export function useQueueUrlState(filters: ModerationQueueFilter[], limit: number) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const search = searchParams.get('search') || '';
  const filterValues: Record<string, string | undefined> = {};
  filters.forEach((filter) => {
    const value = searchParams.get(filter.param);
    filterValues[filter.param] = filter.options.some((o) => o.value !== '' && o.value === value)
      ? (value as string)
      : undefined;
  });

  const state: ModerationQueueState = {
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
    router.push(`${pathname}?${next.toString()}` as any);
  };

  return { state, search, paramsKey: searchParams.toString(), pushParams };
}
