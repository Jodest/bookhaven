import { afterEach, describe, expect, it, vi } from 'vitest';
import { consumeFlash, setFlash } from './flash';

describe('flash', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    window.sessionStorage.clear();
  });

  it('sets, consumes once, then returns null', () => {
    setFlash('catalog.books', 'Книга «X» сохранена.');
    expect(consumeFlash('catalog.books')).toBe('Книга «X» сохранена.');
    expect(consumeFlash('catalog.books')).toBeNull();
  });

  it('keeps keys separate', () => {
    setFlash('catalog.authors', 'a');
    expect(consumeFlash('catalog.genres')).toBeNull();
    expect(consumeFlash('catalog.authors')).toBe('a');
  });

  it('handles a throwing sessionStorage', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });
    expect(() => setFlash('catalog.books', 'x')).not.toThrow();
    expect(consumeFlash('catalog.books')).toBeNull();
  });
});
