'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import type {
  AdminGenreAISuggestion,
  AdminGenreAISuggestionKind,
  AdminGenreAISuggestionStatus,
} from '@bookhaven/api-client';
import { Button, Select, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@bookhaven/ui';
import { GenreStatusBadge } from '@/entities/genre';
import {
  adminGenreAiSuggestionsApi,
  confidencePercent,
  GENRE_AI_STATUS_QUERY_KEY,
  genreAiSuggestionsListQueryKey,
  isSuggestionKind,
  isSuggestionStatus,
  KIND_FILTER_LABELS,
  lastRunText,
  reasonCodeLabel,
  STATUS_TAB_LABELS,
  SUGGESTION_STATUSES,
  suggestionActionLabel,
  suggestionKindLabel,
} from '@/entities/genre-ai-suggestion';
import {
  AnalyzeButton,
  ANALYSIS_DISABLED_TEXT,
  ApplyJunkConfirm,
  ApplySuggestionDialog,
  BulkApplyDialog,
  BulkDismissButton,
  bulkApplyMessage,
  DismissSuggestionButton,
  junkSuccessMessage,
  mergeSuccessMessage,
  type SuggestionFeedback,
} from '@/features/genre-ai-suggestions';
import { Pagination } from '@/shared/ui';

// «Предложения ИИ по жанрам» (openspec catalog-moderation/ai-genre-suggestions):
// the AI only proposes; every merge or rejection here is the admin's action
// through the existing genre merge / bulk status. Deliberately not the shared
// moderation-queue widget: its rows are suggestions, not genres.

const LIMIT = 20;
const REFRESH_WHILE_RUNNING_MS = 10_000;

const KIND_OPTIONS = (Object.keys(KIND_FILTER_LABELS) as ('' | AdminGenreAISuggestionKind)[]).map((value) => ({
  value,
  label: KIND_FILTER_LABELS[value],
}));

type Overlay =
  | { kind: 'merge'; suggestion: AdminGenreAISuggestion }
  | { kind: 'junk'; suggestion: AdminGenreAISuggestion }
  | { kind: 'bulk'; suggestions: AdminGenreAISuggestion[] };

const formatDate = (iso: string) => new Date(iso).toLocaleString('ru-RU');

export function GenreAiSuggestionsView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const rawStatus = searchParams.get('status');
  const status: AdminGenreAISuggestionStatus = isSuggestionStatus(rawStatus) ? rawStatus : 'open';
  const rawKind = searchParams.get('kind');
  const kind: AdminGenreAISuggestionKind | undefined = isSuggestionKind(rawKind) ? rawKind : undefined;
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1);

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [overlay, setOverlay] = useState<Overlay | null>(null);
  const [feedback, setFeedback] = useState<SuggestionFeedback | null>(null);

  const aiStatus = useQuery({
    queryKey: GENRE_AI_STATUS_QUERY_KEY,
    queryFn: () => adminGenreAiSuggestionsApi.status(),
    refetchOnWindowFocus: true,
    refetchInterval: (query) => {
      const run = query.state.data?.last_run;
      return run && (run.status === 'queued' || run.status === 'running') ? REFRESH_WHILE_RUNNING_MS : false;
    },
  });

  const params = { status, kind, page, limit: LIMIT };
  const list = useQuery({
    queryKey: genreAiSuggestionsListQueryKey(params),
    queryFn: () => adminGenreAiSuggestionsApi.list(params),
  });

  const pushParams = (updates: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') next.delete(key);
      else next.set(key, value);
    });
    setFeedback(null);
    setSelected(new Set());
    router.push(`${pathname}?${next.toString()}` as any);
  };

  const items = list.data?.items ?? [];
  const selectable = status === 'open' ? items.filter((s) => !s.stale) : [];
  const selectedItems = selectable.filter((s) => selected.has(s.id));
  const allSelected = selectable.length > 0 && selectedItems.length === selectable.length;
  const totalPages = list.data ? Math.ceil(list.data.total / LIMIT) : 0;
  const enabled = aiStatus.data?.enabled ?? false;
  const run = aiStatus.data?.last_run ?? null;
  const running = run?.status === 'queued' || run?.status === 'running';

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(selectable.map((s) => s.id)));

  const startApply = (suggestion: AdminGenreAISuggestion) => {
    setFeedback(null);
    setOverlay({ kind: suggestion.kind === 'junk' ? 'junk' : 'merge', suggestion });
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-3xl font-bold tracking-tight">Предложения ИИ по жанрам</h1>
          <Link href={'/moderation/genres' as any} className="text-sm text-primary hover:underline">
            ← К модерации жанров
          </Link>
        </div>
        {enabled && <AnalyzeButton running={running} notify={setFeedback} />}
      </div>

      <div className="space-y-1 text-sm text-muted-foreground">
        {aiStatus.data && <p>{lastRunText(run, formatDate)}</p>}
        {aiStatus.data && !enabled && <p>{ANALYSIS_DISABLED_TEXT}</p>}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="flex flex-wrap gap-2" role="group" aria-label="Статус предложений">
          {SUGGESTION_STATUSES.map((s) => (
            <button
              key={s}
              type="button"
              aria-pressed={s === status}
              onClick={() => pushParams({ status: s === 'open' ? undefined : s, page: undefined })}
              className={
                s === status
                  ? 'rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground'
                  : 'rounded-md border px-3 py-1.5 text-sm hover:bg-accent'
              }
            >
              {STATUS_TAB_LABELS[s]}
            </button>
          ))}
        </div>
        <Select
          aria-label="Тип предложения"
          options={KIND_OPTIONS}
          value={kind ?? ''}
          onChange={(e) => pushParams({ kind: e.target.value, page: undefined })}
          className="w-full md:w-48"
        />
      </div>

      {selectedItems.length > 0 && (
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Действия с выбранными">
          <span className="text-sm text-muted-foreground">Выбрано: {selectedItems.length}</span>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setFeedback(null);
              setOverlay({ kind: 'bulk', suggestions: selectedItems });
            }}
          >
            Применить выбранные ({selectedItems.length})
          </Button>
          <BulkDismissButton
            suggestionIds={selectedItems.map((s) => s.id)}
            notify={setFeedback}
            onDone={() => setSelected(new Set())}
          />
        </div>
      )}

      {feedback && (
        <div
          role={feedback.type === 'error' ? 'alert' : 'status'}
          className={`flex items-center justify-between rounded-md p-3 text-sm ${
            feedback.type === 'success' ? 'bg-green-100 text-green-900' : 'bg-destructive/10 text-destructive'
          }`}
        >
          {feedback.message}
          <button type="button" onClick={() => setFeedback(null)} className="ml-4 underline">
            Скрыть
          </button>
        </div>
      )}

      {list.isLoading ? (
        <div className="h-32 animate-pulse rounded-lg bg-muted" data-testid="list-loading" />
      ) : list.isError ? (
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">
          Не удалось загрузить предложения ИИ по жанрам.
        </div>
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {status === 'open' ? 'Открытых предложений нет' : 'Предложений нет'}
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8">
                {status === 'open' && (
                  <input
                    type="checkbox"
                    className="h-4 w-4"
                    aria-label="Выбрать все на странице"
                    checked={allSelected}
                    disabled={selectable.length === 0}
                    onChange={toggleAll}
                  />
                )}
              </TableHead>
              <TableHead>Предложение</TableHead>
              <TableHead>Причина</TableHead>
              <TableHead>Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((s) => (
              <SuggestionRow
                key={s.id}
                suggestion={s}
                selectable={status === 'open' && !s.stale}
                showCheckbox={status === 'open'}
                selected={selected.has(s.id)}
                onToggle={() => toggle(s.id)}
                onApply={() => startApply(s)}
                notify={setFeedback}
              />
            ))}
          </TableBody>
        </Table>
      )}

      <Pagination page={page} totalPages={totalPages} onPageChange={(p) => pushParams({ page: String(p) })} />

      {overlay?.kind === 'merge' && (
        <ApplySuggestionDialog
          suggestion={overlay.suggestion}
          onClose={() => setOverlay(null)}
          onApplied={(result, targetName) => {
            setOverlay(null);
            setSelected(new Set());
            if (result.merge) {
              setFeedback({ type: 'success', message: mergeSuccessMessage(result.merge, targetName) });
            }
          }}
        />
      )}
      {overlay?.kind === 'junk' && (
        <ApplyJunkConfirm
          suggestion={overlay.suggestion}
          onClose={() => setOverlay(null)}
          onApplied={(_result, genreName) => {
            setOverlay(null);
            setSelected(new Set());
            setFeedback({ type: 'success', message: junkSuccessMessage(genreName) });
          }}
        />
      )}
      {overlay?.kind === 'bulk' && (
        <BulkApplyDialog
          suggestions={overlay.suggestions}
          onClose={() => setOverlay(null)}
          onApplied={(report) => {
            setOverlay(null);
            setSelected(new Set());
            setFeedback({ type: 'success', message: bulkApplyMessage(report) });
          }}
        />
      )}
    </div>
  );
}

interface SuggestionRowProps {
  suggestion: AdminGenreAISuggestion;
  selectable: boolean;
  showCheckbox: boolean;
  selected: boolean;
  onToggle: () => void;
  onApply: () => void;
  notify: (feedback: SuggestionFeedback | null) => void;
}

function SuggestionRow({ suggestion: s, selectable, showCheckbox, selected, onToggle, onApply, notify }: SuggestionRowProps) {
  const open = s.status === 'open';
  return (
    <TableRow>
      <TableCell className="align-top">
        {showCheckbox && (
          <input
            type="checkbox"
            className="h-4 w-4"
            aria-label={`Выбрать предложение «${s.genres.map((g) => g.name).join(', ')}»`}
            checked={selected}
            disabled={!selectable}
            onChange={onToggle}
          />
        )}
      </TableCell>
      <TableCell className="align-top">
        <div className="mb-2 flex flex-wrap items-center gap-2">
          <span className="rounded-md bg-muted px-2 py-0.5 text-xs font-medium">{suggestionKindLabel(s.kind)}</span>
          {open && s.stale && (
            <span className="rounded-md bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-900">устарело</span>
          )}
          <span className="text-sm font-medium">
            {suggestionActionLabel(s.kind, s.applied_target?.name ?? s.target?.name ?? null)}
          </span>
        </div>
        <ul className="space-y-1">
          {s.genres.map((g) => (
            <li key={g.id} className="text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span className={g.exists ? 'font-medium' : 'font-medium line-through'}>{g.name}</span>
                {g.exists && g.status !== '' && <GenreStatusBadge status={g.status} />}
                {!g.exists && <span className="text-xs text-muted-foreground">удалён</span>}
                {g.exists && g.merged && <span className="text-xs text-muted-foreground">слит</span>}
                <span className="text-xs text-muted-foreground">книг: {g.books_total}</span>
              </div>
              {g.sample_books.length > 0 && (
                <div className="text-xs text-muted-foreground">
                  {g.sample_books.map((b) => b.title).join(' · ')}
                </div>
              )}
            </li>
          ))}
        </ul>
      </TableCell>
      <TableCell className="max-w-xs align-top text-sm">
        <p>{s.reason}</p>
        <p className="text-xs text-muted-foreground">
          {s.reason_codes.map(reasonCodeLabel).join(', ')} · уверенность {confidencePercent(s.confidence)}
        </p>
        {s.decided_at && (
          <p className="text-xs text-muted-foreground">Решение: {formatDate(s.decided_at)}</p>
        )}
      </TableCell>
      <TableCell className="align-top">
        {open && (
          <div className="flex flex-wrap gap-1">
            <Button size="sm" variant="outline" disabled={s.stale} onClick={onApply}>
              Применить
            </Button>
            <DismissSuggestionButton suggestionId={s.id} notify={notify} />
          </div>
        )}
      </TableCell>
    </TableRow>
  );
}
