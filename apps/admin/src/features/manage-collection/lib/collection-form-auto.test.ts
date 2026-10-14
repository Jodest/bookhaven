import { describe, expect, it } from 'vitest';
import {
  collectionToForm,
  EMPTY_COLLECTION_FORM,
  formToCreateInput,
  formToUpdateInput,
  metricPatch,
  validateRule,
} from './collection-form';

describe('automatic collection form', () => {
  const auto = { ...EMPTY_COLLECTION_FORM, kind: 'auto' as const, title: 'Топ' };

  it('defaults to the weighted rating with 3 / 10', () => {
    expect(auto.metric).toBe('weighted_rating');
    expect(auto.min_ratings).toBe('3');
    expect(auto.confidence).toBe('10');
    expect(formToCreateInput(auto)).toEqual({
      title: 'Топ',
      size: 100,
      is_published: false,
      kind: 'auto',
      rule: { metric: 'weighted_rating', period_days: null, min_ratings: 3, confidence: 10 },
    });
  });

  it('sends min ratings and confidence for the weighted rating only', () => {
    const input = formToCreateInput({ ...auto, metric: 'most_read', period_days: '7' });
    expect(input.rule).toEqual({ metric: 'most_read', period_days: 7 });
  });

  it('a manual collection sends neither kind nor rule', () => {
    const input = formToCreateInput({ ...EMPTY_COLLECTION_FORM, title: 'Топ' });
    expect(input).not.toHaveProperty('kind');
    expect(input).not.toHaveProperty('rule');
  });

  it('update resends the whole rule and never the kind', () => {
    const input = formToUpdateInput({ ...auto, slug: 'top', metric: 'trending', period_days: '30' });
    expect(input.rule).toEqual({ metric: 'trending', period_days: 30 });
    expect(input).not.toHaveProperty('kind');
    expect(formToUpdateInput({ ...EMPTY_COLLECTION_FORM, slug: 'top' })).not.toHaveProperty('rule');
  });

  it('trending prefills 30 days only when the period is empty', () => {
    expect(metricPatch(auto, 'trending')).toEqual({ metric: 'trending', period_days: '30' });
    expect(metricPatch({ ...auto, period_days: '14' }, 'trending')).toEqual({ metric: 'trending' });
    expect(metricPatch(auto, 'most_wanted')).toEqual({ metric: 'most_wanted' });
  });

  it('validates the ranges client-side', () => {
    expect(validateRule(auto)).toBeNull();
    expect(validateRule({ ...auto, period_days: '0' })).toBe('Период: от 1 до 3650 дней');
    expect(validateRule({ ...auto, period_days: '3651' })).toBe('Период: от 1 до 3650 дней');
    expect(validateRule({ ...auto, min_ratings: '' })).toBe('Минимум оценок: от 1 до 10000');
    expect(validateRule({ ...auto, confidence: '10001' })).toBe('Порог доверия: от 1 до 10000');
    expect(validateRule({ ...auto, metric: 'most_read', min_ratings: '' })).toBeNull();
    expect(validateRule({ ...EMPTY_COLLECTION_FORM, period_days: '0' })).toBeNull();
  });

  it('maps a stored rule back to the form', () => {
    const values = collectionToForm({
      id: 'c',
      slug: 's',
      title: 'T',
      description: '',
      kind: 'auto',
      size: 10,
      source: null,
      source_url: null,
      genre: null,
      is_published: false,
      published_at: null,
      rule: { metric: 'trending', period_days: 30, min_ratings: null, confidence: null },
      computed_at: null,
      baseline_at: null,
      created_at: '',
      updated_at: '',
    });
    expect(values).toMatchObject({
      kind: 'auto',
      metric: 'trending',
      period_days: '30',
      min_ratings: '3',
      confidence: '10',
    });
  });
});
