import { describe, expect, it } from 'vitest';
import {
  countInFlightHarvestJobs,
  harvestJobStateBadgeVariant,
  isHarvestJobInFlight,
} from './job-state';

describe('isHarvestJobInFlight', () => {
  it.each(['active', 'retry', 'pending'])('is true for %s', (state) => {
    expect(isHarvestJobInFlight({ state })).toBe(true);
  });

  it.each(['completed', 'archived', 'unknown'])('is false for %s', (state) => {
    expect(isHarvestJobInFlight({ state })).toBe(false);
  });
});

describe('countInFlightHarvestJobs', () => {
  it('returns 0 for undefined', () => {
    expect(countInFlightHarvestJobs(undefined)).toBe(0);
  });

  it('counts only active/retry/pending jobs in a mixed list', () => {
    expect(
      countInFlightHarvestJobs([
        { state: 'active' },
        { state: 'retry' },
        { state: 'pending' },
        { state: 'completed' },
        { state: 'archived' },
        { state: 'pending' },
      ])
    ).toBe(4);
  });
});

describe('harvestJobStateBadgeVariant', () => {
  it.each([
    ['active', 'default'],
    ['pending', 'secondary'],
    ['retry', 'secondary'],
    ['archived', 'destructive'],
    ['completed', 'outline'],
    ['something-else', 'secondary'],
  ])('maps %s to %s', (state, variant) => {
    expect(harvestJobStateBadgeVariant(state)).toBe(variant);
  });
});
