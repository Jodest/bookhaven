import { useQuery } from '@tanstack/react-query';
import { adminDashboardApi, DASHBOARD_QUERY_KEY } from '../api';

interface UseDashboardStatsOptions {
  /** false -> no request at all (non-admin users; the endpoint is admin-only). */
  enabled?: boolean;
}

/**
 * Admin dashboard stats, polled every 60 s (foreground only) and refetched on
 * window focus when stale. Single entry point for DASHBOARD_QUERY_KEY.
 */
export function useDashboardStats({ enabled = true }: UseDashboardStatsOptions = {}) {
  return useQuery({
    queryKey: DASHBOARD_QUERY_KEY,
    queryFn: () => adminDashboardApi.get(),
    enabled,
    refetchInterval: 60_000,
    refetchOnWindowFocus: true,
  });
}
