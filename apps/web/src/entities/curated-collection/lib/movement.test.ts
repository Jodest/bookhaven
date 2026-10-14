import { describe, expect, it } from 'vitest';
import { formatMovement } from './movement';

describe('formatMovement', () => {
  it('formats up, down and new', () => {
    expect(formatMovement({ direction: 'up', by: 3 })).toEqual({ text: '↑3', label: 'Поднялась на 3' });
    expect(formatMovement({ direction: 'down', by: 2 })).toEqual({ text: '↓2', label: 'Опустилась на 2' });
    expect(formatMovement({ direction: 'new', by: 0 })).toEqual({ text: 'Новинка', label: 'Новинка в подборке' });
  });

  it('shows nothing for same, null or undefined', () => {
    expect(formatMovement({ direction: 'same', by: 0 })).toBeNull();
    expect(formatMovement(null)).toBeNull();
    expect(formatMovement(undefined)).toBeNull();
  });
});
