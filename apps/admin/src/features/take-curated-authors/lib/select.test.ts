import { describe, expect, it } from 'vitest';
import type { AdminCurationCandidate, AdminCurationOption } from '@bookhaven/api-client';
import { harvestLimitFor, isOptionSelectable, selectAllMatched } from './select';

function option(qid: string, existing: AdminCurationOption['existing'] = null): AdminCurationOption {
  return { qid, label: qid, description: '', existing };
}

function candidate(
  id: string,
  match_status: AdminCurationCandidate['match_status'],
  options: AdminCurationOption[]
): AdminCurationCandidate {
  return { id, position: 0, name: id, note: '', match_status, options };
}

describe('selectAllMatched', () => {
  it('ticks only new authors of matched candidates', () => {
    const candidates = [
      candidate('new', 'matched', [option('Q1')]),
      candidate('in-catalog', 'matched', [option('Q2', { id: 'a2', name: 'x', slug: 'x', status: 'published' })]),
      candidate('rejected', 'matched', [option('Q3', { id: 'a3', name: 'y', slug: 'y', status: 'rejected' })]),
      candidate('ambiguous', 'ambiguous', [option('Q4'), option('Q5')]),
      candidate('not-found', 'not_found', []),
      candidate('pending', 'pending', []),
      candidate('dup', 'matched', [option('Q1')]),
    ];
    expect(selectAllMatched(candidates)).toEqual(['Q1']);
  });

  it('rejected options are not selectable', () => {
    expect(isOptionSelectable(option('Q1'))).toBe(true);
    expect(isOptionSelectable(option('Q2', { id: 'a', name: 'a', slug: 'a', status: 'pending' }))).toBe(true);
    expect(isOptionSelectable(option('Q3', { id: 'a', name: 'a', slug: 'a', status: 'rejected' }))).toBe(false);
  });

  it('harvest limit is 300 per author, clamped to 5000', () => {
    expect(harvestLimitFor(1)).toBe(300);
    expect(harvestLimitFor(3)).toBe(900);
    expect(harvestLimitFor(20)).toBe(5000);
  });
});
