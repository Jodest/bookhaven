import { useDashboardStats } from '@/entities/dashboard';
import { countInFlightHarvestJobs, useHarvestJobs } from '@/entities/harvest';
import { isAdmin, useAuthStore } from '@/entities/session';
import { buildNavCounters, type NavCounters } from '../lib/build-nav-counters';

const EMPTY_COUNTERS: NavCounters = { sections: {}, tabs: {} };

/**
 * Badge counts for the sidebar and section tabs. Admin-only: for anyone else
 * (moderator, loading session, no user) both queries are disabled, so no
 * request reaches the admin-only endpoints and no 403 can set forbiddenPath.
 * Must be called before any early return (hooks order).
 */
export function useNavCounters(): NavCounters {
  const { user } = useAuthStore();
  const enabled = isAdmin(user);

  const statsQuery = useDashboardStats({ enabled });
  const jobsQuery = useHarvestJobs({
    enabled,
    refetchInterval: (query) =>
      countInFlightHarvestJobs(query.state.data?.items) > 0 ? 10_000 : 60_000,
  });

  if (!enabled) return EMPTY_COUNTERS;
  return buildNavCounters(statsQuery.data, jobsQuery.data?.items);
}
