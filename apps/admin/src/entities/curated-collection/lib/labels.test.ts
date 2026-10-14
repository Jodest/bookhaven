import { describe, expect, it } from 'vitest';
import {
  ITEM_STATE_LABELS,
  KIND_LABELS,
  METRIC_LABELS,
  formatAdminMovement,
  formatScore,
  reserveWarning,
  shortSnapshotWarning,
  shownWarning,
} from './labels';

describe('reserveWarning', () => {
  it('warns when fewer books are shown than places', () => {
    expect(reserveWarning(98, 100)).toBe('В подборке видно 98 из 100 книг — пополните резерв');
  });

  it('is null when the collection is full', () => {
    expect(reserveWarning(100, 100)).toBeNull();
  });
});

describe('ITEM_STATE_LABELS', () => {
  it('has the reserve and hidden markers', () => {
    expect(ITEM_STATE_LABELS.reserve).toBe('Резерв');
    expect(ITEM_STATE_LABELS.hidden).toBe('Скрыта: не опубликована');
  });
});

describe('shortSnapshotWarning', () => {
  it('warns when the rule matches fewer books than places', () => {
    expect(shortSnapshotWarning(40, 100)).toBe('Правилу соответствуют только 40 из 100 книг');
  });

  it('is null when the snapshot is full', () => {
    expect(shortSnapshotWarning(100, 100)).toBeNull();
  });
});

describe('shownWarning', () => {
  it('picks the warning by kind', () => {
    expect(shownWarning('manual', 98, 100)).toBe('В подборке видно 98 из 100 книг — пополните резерв');
    expect(shownWarning('auto', 40, 100)).toBe('Правилу соответствуют только 40 из 100 книг');
    expect(shownWarning('auto', 100, 100)).toBeNull();
  });
});

describe('labels', () => {
  it('names kinds and metrics', () => {
    expect(KIND_LABELS).toEqual({ manual: 'Ручная', auto: 'Авто' });
    expect(METRIC_LABELS.weighted_rating).toBe('Взвешенный рейтинг (оценки читателей)');
    expect(METRIC_LABELS.most_read).toBe('Чаще всего прочитаны');
    expect(METRIC_LABELS.most_wanted).toBe('Чаще всего хотят прочитать');
    expect(METRIC_LABELS.trending).toBe('В тренде (активность за период)');
  });
});

describe('formatAdminMovement', () => {
  it('formats the visible movements', () => {
    expect(formatAdminMovement({ direction: 'up', by: 3 })).toBe('↑3');
    expect(formatAdminMovement({ direction: 'down', by: 2 })).toBe('↓2');
    expect(formatAdminMovement({ direction: 'new', by: 0 })).toBe('Новинка');
  });

  it('shows nothing for same or null', () => {
    expect(formatAdminMovement({ direction: 'same', by: 0 })).toBeNull();
    expect(formatAdminMovement(null)).toBeNull();
  });
});

describe('formatScore', () => {
  it('uses 2 decimals for the weighted rating and a whole number otherwise', () => {
    expect(formatScore('weighted_rating', 4.4512)).toBe('4.45');
    expect(formatScore('most_read', 7)).toBe('7');
    expect(formatScore('trending', 5)).toBe('5');
  });

  it('is empty without a score', () => {
    expect(formatScore('weighted_rating', null)).toBe('');
  });
});
