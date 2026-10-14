import { describe, expect, it } from 'vitest';
import { pasteBook, pasteLine } from './fixtures';
import {
  deriveSelection,
  initialSelection,
  isPickable,
  notFoundText,
  overCapacity,
  selectionReducer,
} from './selection';

const lines = [
  pasteLine(1, 'found', { book: pasteBook('a') }),
  pasteLine(2, 'ambiguous', { candidates: [pasteBook('b'), pasteBook('c'), pasteBook('r', { status: 'rejected' })] }),
  pasteLine(3, 'found', { book: pasteBook('a') }),
  pasteLine(4, 'not_found', { text: 'Нет такой — Никто' }),
  pasteLine(6, 'not_found', { text: 'Ещё одна' }),
  pasteLine(7, 'in_collection', { book: pasteBook('z', { in_collection: true }) }),
];

describe('initialSelection', () => {
  it('ticks found rows only', () => {
    const sel = initialSelection(lines);
    expect(sel.ticked).toEqual({ 1: true, 3: true });
    expect(sel.picks).toEqual({});
  });
});

describe('selectionReducer', () => {
  const reduce = selectionReducer(lines);

  it('picking a candidate ticks its row', () => {
    const sel = reduce(initialSelection(lines), { type: 'pick', line: 2, bookId: 'c' });
    expect(sel.picks[2]).toBe('c');
    expect(sel.ticked[2]).toBe(true);
  });

  it('ignores rejected or in-collection candidates', () => {
    const start = initialSelection(lines);
    expect(reduce(start, { type: 'pick', line: 2, bookId: 'r' })).toBe(start);
    expect(isPickable(pasteBook('x', { in_collection: true }))).toBe(false);
  });

  it('toggles found rows and ignores unresolved or non-tickable rows', () => {
    const start = initialSelection(lines);
    expect(reduce(start, { type: 'toggle', line: 1 }).ticked[1]).toBe(false);
    expect(reduce(start, { type: 'toggle', line: 2 })).toBe(start);
    expect(reduce(start, { type: 'toggle', line: 4 })).toBe(start);
    expect(reduce(start, { type: 'toggle', line: 7 })).toBe(start);
  });
});

describe('deriveSelection', () => {
  const reduce = selectionReducer(lines);

  it('orders ids by line and de-duplicates by book, first row winning', () => {
    const sel = reduce(initialSelection(lines), { type: 'pick', line: 2, bookId: 'b' });
    const d = deriveSelection(lines, sel);
    expect(d.bookIds).toEqual(['a', 'b']);
    expect(d.selectedCount).toBe(2);
    expect(d.repeatOf).toEqual({ 3: 1 });
  });

  it('counts nothing for an unpicked ambiguous row', () => {
    expect(deriveSelection(lines, initialSelection(lines)).bookIds).toEqual(['a']);
  });

  it('a picked book repeating a found one is a repeat', () => {
    const sel = reduce(initialSelection(lines), { type: 'pick', line: 2, bookId: 'b' });
    const lines2 = [pasteLine(1, 'found', { book: pasteBook('b') }), ...lines.slice(1)];
    const d = deriveSelection(lines2, selectionReducer(lines2)(sel, { type: 'pick', line: 2, bookId: 'b' }));
    expect(d.repeatOf[2]).toBe(1);
    expect(d.bookIds).toEqual(['b', 'a']);
  });
});

describe('overCapacity and notFoundText', () => {
  it('compares against free places', () => {
    expect(overCapacity(7, 5)).toBe(true);
    expect(overCapacity(5, 5)).toBe(false);
  });

  it('joins not_found lines', () => {
    expect(notFoundText(lines)).toBe('Нет такой — Никто\nЕщё одна');
  });
});
