import type { AdminHarvestJob } from '@bookhaven/api-client';

/** Job states counted as "в работе": running, queued or waiting to retry. */
export const IN_FLIGHT_JOB_STATES = ['active', 'retry', 'pending'] as const;

export function isHarvestJobInFlight(job: Pick<AdminHarvestJob, 'state'>): boolean {
  return (IN_FLIGHT_JOB_STATES as readonly string[]).includes(job.state);
}

export function countInFlightHarvestJobs(items?: Pick<AdminHarvestJob, 'state'>[]): number {
  if (!items) return 0;
  return items.filter(isHarvestJobInFlight).length;
}

export function harvestJobStateBadgeVariant(
  state: string
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (state) {
    case 'active':
      return 'default';
    case 'pending':
    case 'retry':
      return 'secondary';
    case 'archived':
      return 'destructive';
    case 'completed':
      return 'outline';
    default:
      return 'secondary';
  }
}
