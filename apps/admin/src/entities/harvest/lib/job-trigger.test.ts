import { describe, expect, it } from 'vitest';
import { harvestJobAuthorsLabel, harvestJobTriggerLabel } from './job-trigger';

describe('harvestJobTriggerLabel', () => {
  it('labels scheduled jobs «Авто»', () => {
    expect(harvestJobTriggerLabel({ trigger: 'scheduled' })).toBe('Авто');
  });

  it('labels manual jobs «Вручную»', () => {
    expect(harvestJobTriggerLabel({ trigger: 'manual' })).toBe('Вручную');
  });
});

describe('harvestJobAuthorsLabel', () => {
  it('counts the harvested authors', () => {
    expect(harvestJobAuthorsLabel({ author_qids: ['Q7200', 'Q1183296'] })).toBe('Авторов: 2');
  });

  it('is null when no authors are known', () => {
    expect(harvestJobAuthorsLabel({ author_qids: [] })).toBeNull();
  });
});
