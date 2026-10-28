import { describe, expect, it } from 'vitest';
import type { AdminAdaptationSyncResult } from '@bookhaven/api-client';
import {
  adaptationSyncMoreErrors,
  adaptationSyncProgress,
  adaptationSyncReasonLabel,
  adaptationSyncSummary,
  hasAdaptationSyncInFlight,
  isAdaptationSyncJob,
} from './adaptation-sync';

const report = (over: Partial<AdminAdaptationSyncResult> = {}): AdminAdaptationSyncResult => ({
  total: 683,
  processed: 683,
  updated: 40,
  added: 75,
  posters: 0,
  unchanged: 640,
  skipped: 3,
  failed: 0,
  errors: [],
  ...over,
});

describe('adaptation sync helpers', () => {
  it('adds the poster count to the summary only when above 0', () => {
    expect(adaptationSyncSummary({ adaptation_sync: report({ posters: 12 }) })).toBe(
      'Книг: 683 из 683, обновлено: 40, добавлено экранизаций: 75, добавлено постеров: 12, ошибок: 0'
    );
  });

  it('detects catalog sync jobs', () => {
    expect(isAdaptationSyncJob({ kind: 'adaptation_sync' })).toBe(true);
    expect(isAdaptationSyncJob({ kind: 'harvest' })).toBe(false);
  });

  it('builds the completed summary', () => {
    expect(adaptationSyncSummary({ adaptation_sync: report() })).toBe(
      'Книг: 683 из 683, обновлено: 40, добавлено экранизаций: 75, ошибок: 0'
    );
  });

  it('falls back to the last error or a dash without a report', () => {
    expect(adaptationSyncSummary({ adaptation_sync: null, last_error: 'boom' })).toBe('boom');
    expect(adaptationSyncSummary({ adaptation_sync: null })).toBe('—');
  });

  it('computes progress', () => {
    expect(adaptationSyncProgress({ adaptation_sync: report({ processed: 100 }) })).toEqual({
      processed: 100,
      total: 683,
      percent: 15,
    });
    expect(adaptationSyncProgress({ adaptation_sync: report({ total: 0, processed: 0 }) })).toBeNull();
    expect(adaptationSyncProgress({ adaptation_sync: null })).toBeNull();
  });

  it('labels reasons', () => {
    expect(adaptationSyncReasonLabel('wikidata_unavailable')).toBe('Wikidata недоступна');
    expect(adaptationSyncReasonLabel('save_failed')).toBe('Не удалось сохранить экранизации');
  });

  it('counts errors beyond the list', () => {
    const errors = Array.from({ length: 50 }, (_, i) => ({
      book_id: String(i),
      title: 'x',
      reason: 'save_failed' as const,
    }));
    expect(adaptationSyncMoreErrors({ adaptation_sync: report({ failed: 120, errors }) })).toBe('Ещё ошибок: 70');
    expect(adaptationSyncMoreErrors({ adaptation_sync: report({ failed: 50, errors }) })).toBeNull();
    expect(adaptationSyncMoreErrors({ adaptation_sync: null })).toBeNull();
  });

  it('detects an in-flight catalog sync', () => {
    expect(hasAdaptationSyncInFlight([{ kind: 'adaptation_sync', state: 'active' }])).toBe(true);
    expect(hasAdaptationSyncInFlight([{ kind: 'adaptation_sync', state: 'retry' }])).toBe(true);
    expect(hasAdaptationSyncInFlight([{ kind: 'adaptation_sync', state: 'completed' }])).toBe(false);
    expect(hasAdaptationSyncInFlight([{ kind: 'harvest', state: 'active' }])).toBe(false);
    expect(hasAdaptationSyncInFlight(undefined)).toBe(false);
  });
});
