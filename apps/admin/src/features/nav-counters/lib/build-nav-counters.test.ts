import { describe, expect, it } from 'vitest';
import type { AdminDashboard, AdminHarvestJob } from '@bookhaven/api-client';
import { navSections } from '@/shared/config';
import { buildNavCounters } from './build-nav-counters';

const stats: AdminDashboard = {
  books: 100,
  hidden_books: 10,
  authors: 50,
  genres: 5,
  pending_reviews: 42,
  pending_import_review: 4,
  pending_authors: 7,
};

function job(id: string, state: string): AdminHarvestJob {
  return { id, queue: 'harvest', state, type: 'harvest:wikidata_query', max_retry: 3, retried: 0 };
}

const jobs = [
  job('a', 'active'),
  job('b', 'retry'),
  job('c', 'pending'),
  job('d', 'completed'),
  job('e', 'archived'),
];

describe('buildNavCounters', () => {
  it('maps stats and jobs to section and tab counters', () => {
    expect(buildNavCounters(stats, jobs)).toEqual({
      sections: { moderation: 11, import: 3 },
      tabs: { 'moderation.books': 4, 'moderation.authors': 7, 'import.jobs': 3 },
    });
  });

  it('ignores pending_reviews', () => {
    const counters = buildNavCounters({ ...stats, pending_reviews: 1000 }, undefined);
    expect(counters.sections.moderation).toBe(11);
    expect(Object.values(counters.tabs)).not.toContain(1000);
  });

  it('omits keys whose source is undefined', () => {
    expect(buildNavCounters(undefined, undefined)).toEqual({ sections: {}, tabs: {} });
    expect(buildNavCounters(stats, undefined)).toEqual({
      sections: { moderation: 11 },
      tabs: { 'moderation.books': 4, 'moderation.authors': 7 },
    });
    expect(buildNavCounters(undefined, jobs)).toEqual({
      sections: { import: 3 },
      tabs: { 'import.jobs': 3 },
    });
  });

  it('keeps zero counts (badges hide them, not the builder)', () => {
    expect(buildNavCounters(undefined, [job('d', 'completed')])).toEqual({
      sections: { import: 0 },
      tabs: { 'import.jobs': 0 },
    });
  });

  it('uses only section and tab ids that exist in the navigation config', () => {
    const counters = buildNavCounters(stats, jobs);
    const sectionIds = navSections.map((s) => s.id as string);
    const tabIds = navSections.flatMap((s) => s.tabs.map((t) => t.id));
    for (const id of Object.keys(counters.sections)) {
      expect(sectionIds).toContain(id);
    }
    for (const id of Object.keys(counters.tabs)) {
      expect(tabIds).toContain(id);
    }
  });
});
