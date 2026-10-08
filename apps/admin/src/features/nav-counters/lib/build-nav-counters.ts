import type { AdminDashboard, AdminHarvestJob } from '@bookhaven/api-client';
import { pendingModerationTotal } from '@/entities/dashboard';
import { countInFlightHarvestJobs } from '@/entities/harvest';
import type { NavSectionId } from '@/shared/config';

export interface NavCounters {
  sections: Partial<Record<NavSectionId, number>>;
  /** Keyed by NavTab.id from shared/config/navigation.ts. */
  tabs: Partial<Record<string, number>>;
}

/**
 * Maps dashboard stats and the harvest jobs list to sidebar/tab badge counts.
 * Keys are omitted when their source is undefined (loading, error, disabled).
 */
export function buildNavCounters(stats?: AdminDashboard, jobs?: AdminHarvestJob[]): NavCounters {
  const counters: NavCounters = { sections: {}, tabs: {} };
  if (stats) {
    counters.sections.moderation = pendingModerationTotal(stats);
    counters.tabs['moderation.books'] = stats.pending_import_review;
    counters.tabs['moderation.authors'] = stats.pending_authors;
  }
  if (jobs) {
    const inFlight = countInFlightHarvestJobs(jobs);
    counters.sections.import = inFlight;
    counters.tabs['import.jobs'] = inFlight;
  }
  return counters;
}
