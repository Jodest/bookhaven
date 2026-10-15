'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import type { PasteBook, PasteLine } from '@bookhaven/api-client';
import { isPickable, resolvedBookId, type Selection } from '../lib/selection';
import { PASTE_BOOK_STATUS_LABELS, pasteStatusLabel } from '../lib/status-labels';

/** «Title — Authors, year» line of a catalog book. */
export function bookSummary(book: PasteBook): string {
  const authors = book.authors.map((a) => a.name).join(', ');
  const year = book.published_year ? `, ${book.published_year}` : '';
  return `${book.title}${authors ? ` — ${authors}` : ''}${year}`;
}

interface CandidateListProps {
  line: PasteLine;
  picked: string | undefined;
  onPick: (line: number, bookId: string) => void;
}

export function CandidateList({ line, picked, onPick }: CandidateListProps) {
  return (
    <ul className="mt-1 space-y-1">
      {line.candidates.map((c) => {
        const pickable = isPickable(c);
        return (
          <li key={c.id}>
            <label className="flex items-start gap-2 text-sm">
              <input
                type="radio"
                name={`pick-${line.line}`}
                checked={picked === c.id}
                disabled={!pickable}
                onChange={() => onPick(line.line, c.id)}
                aria-label={`Выбрать: ${bookSummary(c)}`}
              />
              <span className={pickable ? undefined : 'text-muted-foreground'}>
                {bookSummary(c)} · {PASTE_BOOK_STATUS_LABELS[c.status]}
                {c.in_collection && ' · Уже в подборке'}
              </span>
            </label>
          </li>
        );
      })}
    </ul>
  );
}

interface MatchReportTableProps<T extends PasteLine> {
  lines: T[];
  selection: Selection;
  repeatOf: Record<number, number>;
  disabled: boolean;
  onToggle: (line: number) => void;
  onPick: (line: number, bookId: string) => void;
  /** Extra content under the row text (e.g. the AI note). */
  renderExtra?: (line: T) => ReactNode;
  /** Leave the not_found rows out (they are listed elsewhere). */
  hideNotFound?: boolean;
}

/**
 * The per-row match report of a bulk paste preview or an AI draft: tick or
 * pick the catalog books to add. Plain text only.
 */
export function MatchReportTable<T extends PasteLine>({
  lines,
  selection,
  repeatOf,
  disabled,
  onToggle,
  onPick,
  renderExtra,
  hideNotFound = false,
}: MatchReportTableProps<T>) {
  const shown = hideNotFound ? lines.filter((l) => l.status !== 'not_found') : lines;
  return (
    <table className="w-full text-left text-sm">
      <tbody>
        {shown.map((l) => {
          const resolved = resolvedBookId(l, selection);
          const tickable = (l.status === 'found' || l.status === 'ambiguous') && !repeatOf[l.line];
          return (
            <tr key={l.line} className="border-b align-top" data-status={l.status}>
              <td className="w-10 p-2">
                {(l.status === 'found' || l.status === 'ambiguous') && (
                  <input
                    type="checkbox"
                    aria-label={`Добавить строку ${l.line}`}
                    checked={Boolean(selection.ticked[l.line]) && resolved !== null && !repeatOf[l.line]}
                    disabled={disabled || !tickable || resolved === null}
                    onChange={() => onToggle(l.line)}
                  />
                )}
              </td>
              <td className="w-12 p-2 text-muted-foreground">{l.line}</td>
              <td className="p-2">
                <div className="font-medium">{l.text}</div>
                {renderExtra?.(l)}
                {l.book && (
                  <div className="text-sm text-muted-foreground">
                    {bookSummary(l.book)} · {PASTE_BOOK_STATUS_LABELS[l.book.status]}
                  </div>
                )}
                {l.status === 'ambiguous' && (
                  <CandidateList line={l} picked={selection.picks[l.line]} onPick={onPick} />
                )}
                {l.status === 'rejected' && l.candidates.length > 0 && (
                  <ul className="mt-1 text-sm text-muted-foreground">
                    {l.candidates.map((c) => (
                      <li key={c.id}>
                        {bookSummary(c)} · {PASTE_BOOK_STATUS_LABELS[c.status]}
                      </li>
                    ))}
                  </ul>
                )}
              </td>
              <td className="w-64 p-2">
                <div>{pasteStatusLabel(l)}</div>
                {repeatOf[l.line] !== undefined && (
                  <div className="text-xs text-muted-foreground">Повтор строки {repeatOf[l.line]}</div>
                )}
                {l.status === 'not_found' && (
                  <Link
                    href="/import/books/manual"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-primary underline"
                  >
                    Добавить в каталог
                  </Link>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
