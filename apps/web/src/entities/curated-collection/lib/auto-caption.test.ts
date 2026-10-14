import { describe, expect, it } from 'vitest';
import type { AutoRule } from '@bookhaven/api-client';
import { autoCaption } from './auto-caption';

const rule = (over: Partial<AutoRule>): AutoRule => ({
  metric: 'weighted_rating',
  period_days: null,
  min_ratings: 3,
  confidence: 10,
  ...over,
});

describe('autoCaption', () => {
  it('names the basis and the update date', () => {
    expect(autoCaption(rule({}), '2026-10-14T01:00:00Z')).toBe(
      'Составлена автоматически по оценкам читателей BookHaven. Обновлено 14 октября 2026.'
    );
  });

  it('adds the period', () => {
    expect(
      autoCaption(rule({ metric: 'trending', period_days: 30, min_ratings: null, confidence: null }), '2026-10-14T01:00:00Z')
    ).toBe('Составлена автоматически по активности читателей за последние 30 дн. Обновлено 14 октября 2026.');
  });

  it('omits the date before the first computation', () => {
    expect(autoCaption(rule({ metric: 'most_read' }), null)).toBe(
      'Составлена автоматически по отметкам «Прочитано».'
    );
    expect(autoCaption(rule({ metric: 'most_wanted', period_days: 7 }), null)).toBe(
      'Составлена автоматически по отметкам «Хочу прочитать» за последние 7 дн.'
    );
  });
});
