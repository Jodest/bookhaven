'use client';

import { FormEvent, useMemo, useState } from 'react';
import Link from 'next/link';
import { useQueryClient } from '@tanstack/react-query';
import type {
  AdminAiCollectionDraft,
  AdminAiCollectionDraftDetail,
  AdminAiCollectionsStatus,
  AiDraftLine,
  BulkAddResult,
} from '@bookhaven/api-client';
import { Button, Input, Label, Textarea } from '@bookhaven/ui';
import {
  deriveSelection,
  initialSelection,
  MatchReportTable,
  overCapacity,
  PASTE_COUNT_LABELS,
  PASTE_COUNT_ORDER,
  resolvedBookId,
  selectionReducer,
  SKIP_REASON_LABELS,
  useBulkAddBooks,
  type Selection,
} from '@/entities/curated-collection';
import { aiDraftQueryKey } from '../api';
import { defaultDraftCount } from '../lib/default-count';
import {
  applyErrorText,
  createDraftErrorText,
  DRAFT_STATUS_LABELS,
  draftErrorCodeLabel,
} from '../lib/draft-error-text';
import { notFoundLines, proposalText } from '../lib/not-found-text';
import {
  isDraftActive,
  useAiCollectionsStatus,
  useAiDraft,
  useAiDrafts,
  useCreateAiDraft,
} from '../model/use-ai-collections';

interface AiDraftPanelProps {
  collectionId: string;
  /** The collection size (the number of ranked places); drives the default count. */
  size: number;
  /** The books the collection holds now. */
  booksCount: number;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
}

// ---------------------------------------------------------------------------
// Brief form
// ---------------------------------------------------------------------------

interface DraftFormProps {
  collectionId: string;
  status: AdminAiCollectionsStatus;
  size: number;
  booksCount: number;
  hasActiveDraft: boolean;
  onCreated: (id: string) => void;
}

function DraftForm({ collectionId, status, size, booksCount, hasActiveDraft, onCreated }: DraftFormProps) {
  const { create, isPending } = useCreateAiDraft(collectionId);
  const [brief, setBrief] = useState('');
  const [count, setCount] = useState(String(defaultDraftCount(size)));
  const [error, setError] = useState<string | null>(null);

  const trimmed = brief.trim();
  const countNum = Number(count);
  const countValid = Number.isInteger(countNum) && countNum >= 1 && countNum <= status.max_count;
  const limitReached = status.used_today >= status.daily_limit;
  const canSubmit =
    !isPending &&
    !hasActiveDraft &&
    !limitReached &&
    trimmed.length > 0 &&
    brief.length <= status.max_brief_length &&
    countValid;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setError(null);
    try {
      const id = await create({ brief: trimmed, count: countNum });
      setBrief('');
      onCreated(id);
    } catch (err) {
      setError(createDraftErrorText(err));
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="ai-draft-brief">Описание подборки для ИИ</Label>
        <Textarea
          id="ai-draft-brief"
          value={brief}
          maxLength={status.max_brief_length}
          onChange={(e) => setBrief(e.target.value)}
          placeholder="Например: лучшая научная фантастика всех времён, по одной книге на автора"
          rows={3}
        />
        <div className="text-right text-xs text-muted-foreground" aria-live="polite">
          {brief.length} / {status.max_brief_length}
        </div>
      </div>
      <div className="flex flex-wrap items-end gap-4">
        <div className="space-y-2">
          <Label htmlFor="ai-draft-count">Сколько книг предложить</Label>
          <Input
            id="ai-draft-count"
            type="number"
            min={1}
            max={status.max_count}
            value={count}
            onChange={(e) => setCount(e.target.value)}
            className="w-28"
          />
        </div>
        <Button type="submit" disabled={!canSubmit}>
          {isPending ? 'Отправка...' : 'Составить черновик'}
        </Button>
        <span className="text-xs text-muted-foreground">
          Сегодня: {status.used_today} из {status.daily_limit}
        </span>
      </div>
      <p className="text-xs text-muted-foreground">Сейчас в подборке книг: {booksCount}.</p>
      {hasActiveDraft && (
        <p className="text-sm text-muted-foreground">
          Черновик для этой подборки уже составляется — новый можно создать после его завершения.
        </p>
      )}
      {!hasActiveDraft && limitReached && (
        <p className="text-sm text-muted-foreground">Лимит черновиков на сегодня исчерпан</p>
      )}
      {error && (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        ИИ предлагает только книги. Добавляются лишь те, что есть в каталоге, и только после вашего выбора;
        отсутствующие в каталоге книги не создаются.
      </p>
    </form>
  );
}

// ---------------------------------------------------------------------------
// Recent drafts
// ---------------------------------------------------------------------------

interface DraftListProps {
  drafts: AdminAiCollectionDraft[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

function DraftList({ drafts, selectedId, onSelect }: DraftListProps) {
  return (
    <ul className="divide-y rounded-md border" aria-label="Недавние черновики">
      {drafts.map((d) => (
        <li key={d.id}>
          <button
            type="button"
            className={`flex w-full flex-wrap items-baseline gap-x-3 gap-y-1 p-2 text-left text-sm hover:bg-muted ${
              d.id === selectedId ? 'bg-muted' : ''
            }`}
            aria-current={d.id === selectedId ? 'true' : undefined}
            onClick={() => onSelect(d.id)}
          >
            <span className="text-muted-foreground">{formatDate(d.created_at)}</span>
            <span className="text-muted-foreground">{d.created_by?.username ?? 'удалённый пользователь'}</span>
            <span className="min-w-0 flex-1 truncate">{d.brief}</span>
            <span className="font-medium">{DRAFT_STATUS_LABELS[d.status]}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}

// ---------------------------------------------------------------------------
// A done draft: report, not-found list, apply
// ---------------------------------------------------------------------------

type DoneReport = NonNullable<AdminAiCollectionDraftDetail['report']>;

interface ApplyResult {
  result: BulkAddResult;
  /** Proposal text per book id, to name skipped rows. */
  labels: Record<string, string>;
}

interface DraftReportViewProps {
  collectionId: string;
  draftId: string;
  report: DoneReport;
  /** Kept by the parent: the report remounts when it is refetched after an apply. */
  applied: ApplyResult | null;
  onApplied: (r: ApplyResult) => void;
  copied: boolean;
  onCopied: () => void;
}

function DraftReportView({ collectionId, draftId, report, applied, onApplied, copied, onCopied }: DraftReportViewProps) {
  const queryClient = useQueryClient();
  const bulk = useBulkAddBooks(collectionId);
  const [selection, setSelection] = useState<Selection>(() => initialSelection(report.lines));
  const [mode, setMode] = useState<'end' | 'at'>('end');
  const [positionText, setPositionText] = useState('1');

  const derived = useMemo(() => deriveSelection(report.lines, selection), [report.lines, selection]);
  const reducer = useMemo(() => selectionReducer(report.lines), [report.lines]);
  const notFound = useMemo(() => report.lines.filter((l) => l.status === 'not_found'), [report.lines]);

  const position = Number(positionText);
  const positionValid = mode === 'end' || (Number.isInteger(position) && position >= 1);
  const selectedCount = derived.selectedCount;
  const tooMany = overCapacity(selectedCount, report.free_slots);
  const error = applyErrorText(bulk.error);

  const copy = async (value: string) => {
    await navigator.clipboard.writeText(value);
    onCopied();
  };

  const apply = () => {
    if (derived.bookIds.length === 0) return;
    const labels: Record<string, string> = {};
    for (const l of report.lines) {
      const id = resolvedBookId(l, selection);
      if (id !== null && labels[id] === undefined) labels[id] = proposalText(l);
    }
    bulk.mutate(
      { bookIds: derived.bookIds, position: mode === 'at' ? position : undefined },
      {
        onSuccess: (result) => {
          onApplied({ result, labels });
          // The report is matched at read time: refetch once so the added
          // books read «Уже в подборке».
          queryClient.invalidateQueries({ queryKey: aiDraftQueryKey(draftId) });
        },
      }
    );
  };

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground" data-testid="ai-draft-counts">
        {PASTE_COUNT_ORDER.filter((s) => report.counts[s] > 0)
          .map((s) => `${PASTE_COUNT_LABELS[s]}: ${report.counts[s]}`)
          .join(' · ')}
      </p>

      <MatchReportTable<AiDraftLine>
        lines={report.lines}
        selection={selection}
        repeatOf={derived.repeatOf}
        disabled={bulk.isPending}
        hideNotFound
        onToggle={(line) => setSelection((s) => reducer(s, { type: 'toggle', line }))}
        onPick={(line, bookId) => setSelection((s) => reducer(s, { type: 'pick', line, bookId }))}
        renderExtra={(l) => (
          <>
            {l.original_title && <div className="text-xs text-muted-foreground">Оригинал: {l.original_title}</div>}
            {l.note && <div className="text-sm text-muted-foreground">{l.note}</div>}
          </>
        )}
      />

      <fieldset className="flex flex-wrap items-center gap-4" disabled={bulk.isPending}>
        <legend className="sr-only">Куда добавить</legend>
        <label className="flex items-center gap-2 text-sm">
          <input type="radio" name="ai-draft-position" checked={mode === 'end'} onChange={() => setMode('end')} />В
          конец
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="radio" name="ai-draft-position" checked={mode === 'at'} onChange={() => setMode('at')} />
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
          disabled={bulk.isPending || selectedCount === 0 || tooMany || !positionValid}
          onClick={apply}
        >
          Добавить выбранные ({selectedCount})
        </Button>
      </div>
      {tooMany && (
        <p className="text-sm text-destructive">Свободно мест: {report.free_slots} — снимите лишние отметки</p>
      )}
      {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}

      {applied && (
        <div className="space-y-2" data-testid="ai-draft-result">
          <p className="font-medium">Добавлено: {applied.result.added.length}</p>
          <p className="font-medium">Пропущено: {applied.result.skipped.length}</p>
          {applied.result.skipped.length > 0 && (
            <ul className="list-disc pl-5 text-sm text-muted-foreground">
              {applied.result.skipped.map((s) => (
                <li key={`${s.book_id}-${s.reason}`}>
                  {applied.labels[s.book_id] ?? s.book_id} — {SKIP_REASON_LABELS[s.reason]}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {notFound.length > 0 && (
        <section className="space-y-2" aria-label="Нет в каталоге">
          <div className="flex flex-wrap items-center gap-3">
            <h4 className="font-medium">Нет в каталоге ({notFound.length})</h4>
            <Button type="button" variant="outline" size="sm" onClick={() => copy(notFoundLines(report.lines))}>
              Скопировать ненайденные
            </Button>
            {copied && <span className="text-sm text-muted-foreground">Скопировано</span>}
          </div>
          <ul className="divide-y rounded-md border">
            {notFound.map((l) => (
              <li key={l.line} className="flex flex-wrap items-center justify-between gap-3 p-2 text-sm">
                <div>
                  <span className="mr-2 text-muted-foreground">{l.line}</span>
                  <span className="font-medium">{proposalText(l)}</span>
                  {l.original_title && (
                    <span className="ml-2 text-xs text-muted-foreground">Оригинал: {l.original_title}</span>
                  )}
                </div>
                <Link
                  href="/import/books/manual"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary underline"
                >
                  Добавить в каталог
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// The opened draft
// ---------------------------------------------------------------------------

interface DraftViewProps {
  collectionId: string;
  draftId: string;
}

function DraftView({ collectionId, draftId }: DraftViewProps) {
  const query = useAiDraft(draftId, collectionId);
  const draft = query.data;
  const [applied, setApplied] = useState<ApplyResult | null>(null);
  const [copied, setCopied] = useState(false);

  if (query.isError) {
    return (
      <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
        Не удалось загрузить черновик.
      </div>
    );
  }
  if (!draft) return <p className="text-sm text-muted-foreground">Загрузка черновика…</p>;

  if (isDraftActive(draft.status)) {
    return (
      <p className="text-sm" role="status">
        ИИ составляет черновик… предложено {draft.proposals_count} из {draft.count}
      </p>
    );
  }
  if (draft.status === 'failed') {
    return (
      <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
        {draftErrorCodeLabel(draft.error_code)}
      </div>
    );
  }
  if (!draft.report) return null;

  return (
    <div className="space-y-3">
      {draft.error_code && (
        <p className="rounded-md bg-muted p-3 text-sm" role="status">
          Черновик составлен не полностью ({draft.proposals_count} из {draft.count}): {draftErrorCodeLabel(draft.error_code)}.
        </p>
      )}
      {draft.report.lines.length === 0 ? (
        <p className="text-sm text-muted-foreground">ИИ не предложил ни одной книги.</p>
      ) : (
        // Re-seeded from the report each time it is refetched (after an apply).
        <DraftReportView
          key={query.dataUpdatedAt}
          collectionId={collectionId}
          draftId={draft.id}
          report={draft.report}
          applied={applied}
          onApplied={setApplied}
          copied={copied}
          onCopied={() => setCopied(true)}
        />
      )}
    </div>
  );
}

/**
 * «Черновик ИИ» of a manual collection: a brief form, the collection recent
 * drafts, and the opened draft with its catalog match report. Books absent
 * from the catalog are only listed, never created or added.
 */
export function AiDraftPanel({ collectionId, size, booksCount }: AiDraftPanelProps) {
  const status = useAiCollectionsStatus();
  const drafts = useAiDrafts(collectionId);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const items = drafts.data?.items ?? [];
  const openId = selectedId ?? items[0]?.id ?? null;
  const hasActiveDraft = items.some((d) => isDraftActive(d.status));

  if (status.isLoading) {
    return (
      <section className="rounded-lg border p-4 text-sm text-muted-foreground" aria-label="Черновик ИИ">
        Загрузка…
      </section>
    );
  }

  return (
    <section className="space-y-4 rounded-lg border p-4" aria-label="Черновик ИИ">
      {status.data && status.data.enabled ? (
        <DraftForm
          collectionId={collectionId}
          status={status.data}
          size={size}
          booksCount={booksCount}
          hasActiveDraft={hasActiveDraft}
          onCreated={setSelectedId}
        />
      ) : (
        <div className="rounded-md border p-4 text-sm">
          <p className="font-medium">Черновики ИИ выключены</p>
          <p className="text-muted-foreground">
            Включите AI_COLLECTIONS_ENABLED и задайте ANTHROPIC_API_KEY для api и воркера.
          </p>
        </div>
      )}

      {items.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium">Недавние черновики</h3>
          <DraftList drafts={items} selectedId={openId} onSelect={setSelectedId} />
        </div>
      )}

      {openId && <DraftView key={openId} collectionId={collectionId} draftId={openId} />}
    </section>
  );
}
