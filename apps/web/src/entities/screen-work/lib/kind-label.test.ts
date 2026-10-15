import { describe, expect, it } from 'vitest';
import { screenWorkKindLabel } from './kind-label';

describe('screenWorkKindLabel', () => {
  it('maps every kind to its Russian label', () => {
    expect(screenWorkKindLabel('film')).toBe('Фильм');
    expect(screenWorkKindLabel('tv_film')).toBe('Телефильм');
    expect(screenWorkKindLabel('series')).toBe('Сериал');
    expect(screenWorkKindLabel('animated_film')).toBe('Мультфильм');
    expect(screenWorkKindLabel('animated_series')).toBe('Мультсериал');
    expect(screenWorkKindLabel('teleplay')).toBe('Телеспектакль');
  });
});
