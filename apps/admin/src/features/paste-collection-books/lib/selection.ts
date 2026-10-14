import type { PasteBook, PasteLine } from '@bookhaven/api-client';

/** Admin choices on top of a preview, keyed by the pasted line number. */
export interface Selection {
  /** Candidate picked on an ambiguous row. */
  picks: Record<number, string>;
  /** Rows the admin ticked (or the default found ticks). */
  ticked: Record<number, boolean>;
}

export type SelectionAction =
  | { type: 'toggle'; line: number }
  | { type: 'pick'; line: number; bookId: string };

/** A candidate can be picked unless it is rejected or already in the collection. */
export function isPickable(book: PasteBook): boolean {
  return book.status !== 'rejected' && !book.in_collection;
}

/** Initial selection: every found row is ticked. */
export function initialSelection(lines: PasteLine[]): Selection {
  const ticked: Record<number, boolean> = {};
  for (const l of lines) {
    if (l.status === 'found' && l.book) ticked[l.line] = true;
  }
  return { picks: {}, ticked };
}

/** The book a row currently stands for, or null when it cannot be added. */
export function resolvedBookId(line: PasteLine, sel: Selection): string | null {
  if (line.status === 'found') return line.book?.id ?? null;
  if (line.status === 'ambiguous') {
    const picked = sel.picks[line.line];
    const cand = line.candidates.find((c) => c.id === picked);
    return cand && isPickable(cand) ? cand.id : null;
  }
  return null;
}

export function selectionReducer(lines: PasteLine[]) {
  return (sel: Selection, action: SelectionAction): Selection => {
    const row = lines.find((l) => l.line === action.line);
    if (!row) return sel;
    if (action.type === 'pick') {
      const cand = row.candidates.find((c) => c.id === action.bookId);
      if (row.status !== 'ambiguous' || !cand || !isPickable(cand)) return sel;
      return {
        picks: { ...sel.picks, [row.line]: cand.id },
        ticked: { ...sel.ticked, [row.line]: true },
      };
    }
    if (resolvedBookId(row, sel) === null) return sel;
    return { ...sel, ticked: { ...sel.ticked, [row.line]: !sel.ticked[row.line] } };
  };
}

export interface DerivedSelection {
  /** Book ids to send, in pasted order, first row winning per book. */
  bookIds: string[];
  selectedCount: number;
  /** Ticked rows dropped because an earlier row has the same book: line -> earlier line. */
  repeatOf: Record<number, number>;
}

/** Ordered, de-duplicated ticked book ids and the rows dropped as repeats. */
export function deriveSelection(lines: PasteLine[], sel: Selection): DerivedSelection {
  const bookIds: string[] = [];
  const firstLine = new Map<string, number>();
  const repeatOf: Record<number, number> = {};
  for (const l of lines) {
    if (!sel.ticked[l.line]) continue;
    const id = resolvedBookId(l, sel);
    if (id === null) continue;
    const earlier = firstLine.get(id);
    if (earlier !== undefined) {
      repeatOf[l.line] = earlier;
      continue;
    }
    firstLine.set(id, l.line);
    bookIds.push(id);
  }
  return { bookIds, selectedCount: bookIds.length, repeatOf };
}

export function overCapacity(selectedCount: number, freeSlots: number): boolean {
  return selectedCount > freeSlots;
}

/** The not_found lines, one per line, ready to be re-pasted. */
export function notFoundText(lines: PasteLine[]): string {
  return lines
    .filter((l) => l.status === 'not_found')
    .map((l) => l.text)
    .join('\n');
}
