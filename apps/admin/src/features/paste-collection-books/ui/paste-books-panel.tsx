'use client';

import { useMemo, useState } from 'react';
import type { BulkAddResult, PastePreview } from '@bookhaven/api-client';
import { Button, Input, Label, Textarea } from '@bookhaven/ui';
import {
  deriveSelection,
  initialSelection,
  MatchReportTable,
  notFoundText,
  overCapacity,
  PASTE_COUNT_LABELS,
  PASTE_COUNT_ORDER,
  resolvedBookId,
  selectionReducer,
  SKIP_REASON_LABELS,
  useBulkAddBooks,
  type Selection,
} from '@/entities/curated-collection';
import { pasteErrorText } from '../lib/paste-error-text';
import { usePastePreview } from '../model/use-paste-books';

interface PasteBooksPanelProps {
  collectionId: string;
}

interface Report {
  result: BulkAddResult;
  /** Pasted text per book id, to name skipped rows. */
  labels: Record<string, string>;
  /** not_found lines of the applied preview, for re-pasting. */
  notFound: string;
}

/**
 * Bulk paste of books into a collection: textarea, per-line preview against
 * the catalog, choice of the books to add and a result report. A line never
 * creates a book: unmatched lines are reported as not found.
 */
export function PasteBooksPanel({ collectionId }: PasteBooksPanelProps) {
  const previewMutation = usePastePreview(collectionId);
  const bulk = useBulkAddBooks(collectionId);

  const [text, setText] = useState('');
  const [preview, setPreview] = useState<PastePreview | null>(null);
  const [selection, setSelection] = useState<Selection>({ picks: {}, ticked: {} });
  const [mode, setMode] = useState<'end' | 'at'>('end');
  const [positionText, setPositionText] = useState('1');
  const [report, setReport] = useState<Report | null>(null);
  const [copied, setCopied] = useState(false);

  const derived = useMemo(
    () => (preview ? deriveSelection(preview.lines, selection) : null),
    [preview, selection]
  );

  const position = Number(positionText);
  const positionValid = mode === 'end' || (Number.isInteger(position) && position >= 1);
  const selectedCount = derived?.selectedCount ?? 0;
  const tooMany = preview ? overCapacity(selectedCount, preview.free_slots) : false;
  const pending = previewMutation.isPending || bulk.isPending;
  const error = pasteErrorText(previewMutation.error ?? bulk.error);

  const onTextChange = (value: string) => {
    setText(value);
    setPreview(null);
    setReport(null);
    setCopied(false);
    previewMutation.reset();
    bulk.reset();
  };

  const runPreview = () => {
    setReport(null);
    setCopied(false);
    bulk.reset();
    previewMutation.mutate(text, {
      onSuccess: (data) => {
        setPreview(data);
        setSelection(initialSelection(data.lines));
      },
    });
  };

  const apply = () => {
    if (!preview || !derived || derived.bookIds.length === 0) return;
    const labels: Record<string, string> = {};
    for (const l of preview.lines) {
      const id = resolvedBookId(l, selection);
      if (id !== null && labels[id] === undefined) labels[id] = l.text;
    }
    bulk.mutate(
      { bookIds: derived.bookIds, position: mode === 'at' ? position : undefined },
      {
        onSuccess: (result) => {
          setReport({ result, labels, notFound: notFoundText(preview.lines) });
          setPreview(null);
        },
      }
    );
  };

  const reducer = preview ? selectionReducer(preview.lines) : null;

  const copyText = async (value: string) => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
  };

  return (
    <section className="space-y-4 rounded-lg border p-4" aria-label="Вставка списка книг">
      <div className="space-y-2">
        <Label htmlFor="paste-books-text">Список книг</Label>
        <Textarea
          id="paste-books-text"
          rows={8}
          value={text}
          onChange={(e) => onTextChange(e.target.value)}
        />
        <p className="text-xs text-muted-foreground">
          Одна книга в строке: Название — Автор. Нумерация и кавычки допускаются. До 500 строк.
        </p>
        <Button type="button" variant="outline" disabled={pending || text.trim() === ''} onClick={runPreview}>
          Найти в каталоге
        </Button>
      </div>

      {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}

      {preview && derived && reducer && (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground" data-testid="paste-counts">
            {PASTE_COUNT_ORDER.filter((s) => preview.counts[s] > 0)
              .map((s) => `${PASTE_COUNT_LABELS[s]}: ${preview.counts[s]}`)
              .join(' · ')}
          </p>
          <MatchReportTable
            lines={preview.lines}
            selection={selection}
            repeatOf={derived.repeatOf}
            disabled={pending}
            onToggle={(line) => setSelection((s) => reducer(s, { type: 'toggle', line }))}
            onPick={(line, bookId) => setSelection((s) => reducer(s, { type: 'pick', line, bookId }))}
          />

          <fieldset className="flex flex-wrap items-center gap-4" disabled={pending}>
            <legend className="sr-only">Куда добавить</legend>
            <label className="flex items-center gap-2 text-sm">
              <input type="radio" name="paste-position" checked={mode === 'end'} onChange={() => setMode('end')} />В конец
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="radio" name="paste-position" checked={mode === 'at'} onChange={() => setMode('at')} />
              На позицию
            </label>
            {mode === 'at' && (
              <Input
                type="number"
                min={1}
                className="h-8 w-24"
                aria-label="Позиция"
                value={positionText}
                onChange={(e) => setPositionText(e.target.value)}
              />
            )}
          </fieldset>

          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              disabled={pending || selectedCount === 0 || tooMany || !positionValid}
              onClick={apply}
            >
              Добавить выбранные ({selectedCount})
            </Button>
            {preview.counts.not_found > 0 && (
              <Button type="button" variant="outline" onClick={() => copyText(notFoundText(preview.lines))}>
                Скопировать ненайденные
              </Button>
            )}
            {copied && <span className="text-sm text-muted-foreground">Скопировано</span>}
          </div>
          {tooMany && (
            <p className="text-sm text-destructive">
              Свободно мест: {preview.free_slots} — снимите лишние отметки
            </p>
          )}
        </div>
      )}

      {report && (
        <div className="space-y-2" data-testid="paste-report">
          <p className="font-medium">Добавлено: {report.result.added.length}</p>
          <p className="font-medium">Пропущено: {report.result.skipped.length}</p>
          {report.result.skipped.length > 0 && (
            <ul className="list-disc pl-5 text-sm text-muted-foreground">
              {report.result.skipped.map((s) => (
                <li key={`${s.book_id}-${s.reason}`}>
                  {report.labels[s.book_id] ?? s.book_id} — {SKIP_REASON_LABELS[s.reason]}
                </li>
              ))}
            </ul>
          )}
          {report.notFound !== '' && (
            <div className="flex items-center gap-3">
              <Button type="button" variant="outline" onClick={() => copyText(report.notFound)}>
                Скопировать ненайденные
              </Button>
              {copied && <span className="text-sm text-muted-foreground">Скопировано</span>}
            </div>
          )}
        </div>
      )}
    </section>
  );
}
