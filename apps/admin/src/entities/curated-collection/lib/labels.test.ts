import { describe, expect, it } from 'vitest';
import { ITEM_STATE_LABELS, reserveWarning } from './labels';

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
