import { describe, expect, it } from 'vitest';
import { getPaginationRange } from './pagination-range';

const fmt = (current: number, total: number, siblings?: number) =>
  getPaginationRange(current, total, siblings)
    .map((i) => (i.type === 'page' ? String(i.page) : '…'))
    .join(' ');

describe('getPaginationRange', () => {
  it('handles empty and tiny totals', () => {
    expect(getPaginationRange(1, 0)).toEqual([]);
    expect(getPaginationRange(1, -3)).toEqual([]);
    expect(fmt(1, 1)).toBe('1');
  });

  it('lists all pages when total <= 2*siblings + 5', () => {
    expect(fmt(3, 7)).toBe('1 2 3 4 5 6 7');
    expect(fmt(1, 7, 1)).toBe('1 2 3 4 5 6 7');
  });

  it('collapses the end, middle and start', () => {
    expect(fmt(1, 42)).toBe('1 2 3 4 5 … 42');
    expect(fmt(5, 42)).toBe('1 … 4 5 6 … 42');
    expect(fmt(42, 42)).toBe('1 … 38 39 40 41 42');
  });

  it('clamps current into range', () => {
    expect(fmt(0, 42)).toBe(fmt(1, 42));
    expect(fmt(100, 42)).toBe(fmt(42, 42));
  });

  it('never uses an ellipsis for a single page', () => {
    for (let total = 1; total <= 30; total++) {
      for (let cur = 1; cur <= total; cur++) {
        const items = getPaginationRange(cur, total);
        const pages = items.filter((i) => i.type === 'page').map((i) => (i as { page: number }).page);
        items.forEach((it, idx) => {
          if (it.type === 'ellipsis') {
            const before = (items[idx - 1] as { page: number }).page;
            const after = (items[idx + 1] as { page: number }).page;
            expect(after - before).toBeGreaterThan(2);
          }
        });
        expect(pages[0]).toBe(1);
        expect(pages[pages.length - 1]).toBe(total);
        expect(pages).toContain(cur);
      }
    }
  });

  it('supports siblings = 2', () => {
    expect(fmt(10, 42, 2)).toBe('1 … 8 9 10 11 12 … 42');
    expect(fmt(1, 42, 2)).toBe('1 2 3 4 5 6 7 … 42');
    expect(fmt(5, 9, 2)).toBe('1 2 3 4 5 6 7 8 9');
  });
});
