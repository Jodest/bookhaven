import { useQuery, type Query } from '@tanstack/react-query';
import type { AdminHarvestJob } from '@bookhaven/api-client';
import { adminHarvestApi, HARVEST_JOBS_QUERY_KEY } from '../api';

type HarvestJobsData = { items: AdminHarvestJob[] };

interface UseHarvestJobsOptions {
  /** false -> no request at all (non-admin users; the endpoint is admin-only). */
  enabled?: boolean;
  refetchInterval?:
    | number
    | false
    | ((query: Query<HarvestJobsData, Error, HarvestJobsData, readonly unknown[]>) => number | false | undefined);
}

/**
 * Harvest jobs list (GET /admin/harvest/jobs). Single entry point for
 * HARVEST_JOBS_QUERY_KEY: every observer (jobs page, dashboard, nav counters)
 * shares one cache entry; TanStack polls at the smallest observer interval.
 */
export function useHarvestJobs({ enabled = true, refetchInterval }: UseHarvestJobsOptions = {}) {
  return useQuery({
    queryKey: HARVEST_JOBS_QUERY_KEY,
    queryFn: () => adminHarvestApi.listJobs(),
    enabled,
    refetchInterval,
    refetchOnWindowFocus: true,
  });
}
