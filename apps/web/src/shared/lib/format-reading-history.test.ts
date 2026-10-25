import type { BookRead } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import {
  formatReadingLine,
  toLocalDateInput,
  todayLocalDateInput,
} from './format-reading-history';

const local = (y: number, m: number, d: number) => new Date(y, m - 1, d, 12).toISOString();

function read(partial: Partial<BookRead>): BookRead {
  return {
    id: 'r',
    bookId: 'b',
    state: 'finished',
    startedAt: null,
    finishedAt: null,
    abandonedAt: null,
    ...partial,
  };
}

describe('formatReadingLine', () => {
  it('finished with a start', () => {
    expect(
      formatReadingLine(1, read({ startedAt: local(2019, 3, 5), finishedAt: local(2019, 4, 20) }))
    ).toBe('1-е прочтение: март 2019 — апрель 2019');
  });

  it('finished without a start', () => {
    expect(formatReadingLine(2, read({ finishedAt: local(2015, 4, 20) }))).toBe(
      '2-е прочтение: апрель 2015'
    );
  });

  it('abandoned', () => {
    expect(
      formatReadingLine(
        2,
        read({ state: 'abandoned', startedAt: local(2023, 1, 5), abandonedAt: local(2023, 2, 1) })
      )
    ).toBe('2-е прочтение: январь 2023 — февраль 2023, брошено');
    expect(formatReadingLine(2, read({ state: 'abandoned', abandonedAt: local(2023, 2, 1) }))).toBe(
      '2-е прочтение: февраль 2023, брошено'
    );
  });

  it('open uses the genitive month', () => {
    expect(formatReadingLine(3, read({ state: 'open', startedAt: local(2026, 5, 10) }))).toBe(
      '3-е прочтение: с мая 2026 — читаю'
    );
  });

  it('uses nominative months across the year boundary', () => {
    expect(
      formatReadingLine(1, read({ startedAt: local(2015, 12, 31), finishedAt: local(2016, 1, 1) }))
    ).toBe('1-е прочтение: декабрь 2015 — январь 2016');
  });

  it.each([
    [1, 'января'],
    [2, 'февраля'],
    [3, 'марта'],
    [4, 'апреля'],
    [5, 'мая'],
    [6, 'июня'],
    [7, 'июля'],
    [8, 'августа'],
    [9, 'сентября'],
    [10, 'октября'],
    [11, 'ноября'],
    [12, 'декабря'],
  ])('genitive month %i', (m, name) => {
    expect(formatReadingLine(1, read({ state: 'open', startedAt: local(2026, m, 10) }))).toBe(
      `1-е прочтение: с ${name} 2026 — читаю`
    );
  });
});

describe('date input helpers', () => {
  it('formats the local day', () => {
    expect(toLocalDateInput(local(2015, 12, 31))).toBe('2015-12-31');
    expect(toLocalDateInput(local(2020, 2, 3))).toBe('2020-02-03');
  });

  it("returns today's local day", () => {
    const n = new Date();
    expect(todayLocalDateInput()).toBe(
      `${n.getFullYear()}-${String(n.getMonth() + 1).padStart(2, '0')}-${String(n.getDate()).padStart(2, '0')}`
    );
  });
});
