'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@bookhaven/api-client';
import {
  Button,
  Input,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Search } from 'lucide-react';
import { consumeFlash } from '@/shared/lib';
import { Pagination } from '@/shared/ui';
import { useQueueUrlState } from '../model/use-queue-url-state';
import type {
  ModerationFeedback,
  ModerationQueueColumn,
  ModerationQueueControls,
  ModerationQueueFilter,
  ModerationQueueProps,
  ModerationRowContext,
  ModerationStatus,
  ModerationStatusReport,
} from '../model/types';

// Shared moderation queue shell (docs/features/moderation-queue-widget.md):
// URL state, status tabs, search + filters, selection, bulk bar, per-row
// status buttons with confirm, feedback banner, states, table, pagination.

const DEFAULT_LIMIT = 20;
const NO_FILTERS: ModerationQueueFilter[] = [];

const STATUS_TABS: { value: ModerationStatus; label: string }[] = [
  { value: 'pending', label: 'На модерации' },
  { value: 'published', label: 'Опубликованные' },
  { value: 'rejected', label: 'Отклонённые' },
];

export function ModerationQueue<TItem, TParams, TReport extends ModerationStatusReport>({
  queryKey,
  buildParams,
  queryFn,
  statusMutation,
  getId,
  getName,
  getStatus,
  filters = NO_FILTERS,
  columns,
  renderRowActions,
  renderBulkActions,
  renderOverlay,
  renderToolbar,
  copy,
  limit = DEFAULT_LIMIT,
  flashKey,
}: ModerationQueueProps<TItem, TParams, TReport>) {
  const { state, search, paramsKey, pushParams } = useQueueUrlState(filters, limit);
  const { status, page } = state;

  const [searchInput, setSearchInput] = useState(search);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [feedback, setFeedback] = useState<ModerationFeedback | null>(null);

  // Selection belongs to the current page and filters only.
  useEffect(() => {
    setSelected(new Set());
  }, [paramsKey]);

  // One-shot success message left by an edit page (opt-in via flashKey).
  // Only a non-null message touches the banner (strict-mode double effect).
  useEffect(() => {
    if (!flashKey) return;
    const message = consumeFlash(flashKey);
    if (message) {
      setFeedback({ type: 'success', message });
    }
  }, [flashKey]);

  const params = buildParams(state);
  const { data, isLoading, error } = useQuery({
    queryKey: [...queryKey, params],
    queryFn: () => queryFn(params),
  });

  const items = data?.items ?? [];
  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  const updateParams = (updates: Record<string, string | undefined>) => {
    setSelected(new Set());
    pushParams(updates);
  };

  const controls: ModerationQueueControls = {
    notify: setFeedback,
    clearSelection: () => setSelected(new Set()),
    select: (ids) => {
      const onPage = new Set(items.map(getId));
      setSelected(new Set(ids.filter((id) => onPage.has(id))));
    },
  };

  const errorText = (err: unknown) =>
    err instanceof ApiError && err.message ? err.message : copy.statusErrorFallback;

  const handleRowStatus = (item: TItem, next: ModerationStatus) => {
    setFeedback(null);
    if (!confirm(copy.rowConfirm(next, item))) {
      return;
    }
    const name = getName(item);
    statusMutation.mutate(
      { ids: [getId(item)], status: next },
      {
        onSuccess: (report) => {
          if (report.errors.length > 0) {
            setFeedback({
              type: 'error',
              message: `Не удалось изменить статус «${name}»: ${report.errors[0].message}.`,
            });
            return;
          }
          setFeedback({ type: 'success', message: copy.rowSuccess(next, name) });
        },
        onError: (err) => setFeedback({ type: 'error', message: errorText(err) }),
      }
    );
  };

  const selectedItems = items.filter((i) => selected.has(getId(i)));

  const handleBulkStatus = (next: ModerationStatus) => {
    setFeedback(null);
    if (selectedItems.length === 0) return;
    if (!confirm(copy.bulkConfirm(next, selectedItems))) {
      return;
    }
    const names = Object.fromEntries(selectedItems.map((i) => [getId(i), getName(i)]));
    statusMutation.mutate(
      { ids: selectedItems.map(getId), status: next },
      {
        onSuccess: (report) => {
          setSelected(new Set());
          setFeedback({
            type: report.errors.length > 0 ? 'error' : 'success',
            message: copy.bulkSummary(report, names),
          });
        },
        onError: (err) => setFeedback({ type: 'error', message: errorText(err) }),
      }
    );
  };

  const toggleRow = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const allSelected = items.length > 0 && items.every((i) => selected.has(getId(i)));
  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(items.map(getId)));
  };

  const hasExtraFilters = Boolean(search || Object.values(state.filters).some(Boolean));
  const emptyText = status === 'pending' && !hasExtraFilters ? copy.emptyPending : copy.emptyFiltered;
  const pending = statusMutation.isPending;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">{copy.title}</h1>

      <div className="flex flex-wrap gap-2" role="group" aria-label="Статус">
        {STATUS_TABS.map((tab) => (
          <Button
            key={tab.value}
            type="button"
            variant={status === tab.value ? 'default' : 'outline'}
            aria-pressed={status === tab.value}
            onClick={() =>
              updateParams({ status: tab.value === 'pending' ? undefined : tab.value, page: undefined })
            }
          >
            {tab.label}
          </Button>
        ))}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          updateParams({ search: searchInput.trim(), page: undefined });
        }}
        className="flex flex-col gap-2 md:flex-row"
      >
        <Input
          placeholder={copy.searchPlaceholder}
          aria-label="Поиск"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
        <Button type="submit" variant="outline" aria-label="Найти">
          <Search className="h-4 w-4" />
        </Button>
        {filters.map((filter) => (
          <Select
            key={filter.param}
            aria-label={filter.label}
            options={filter.options}
            value={state.filters[filter.param] ?? ''}
            onChange={(e) => updateParams({ [filter.param]: e.target.value, page: undefined })}
            className="w-full md:w-40"
          />
        ))}
      </form>

      {renderToolbar?.(items, state, controls)}

      {feedback && (
        <div
          className={`flex items-center justify-between rounded-md p-3 text-sm ${
            feedback.type === 'success'
              ? 'bg-green-100 text-green-900'
              : 'bg-destructive/10 text-destructive'
          }`}
        >
          {feedback.action ? (
            <span>
              {feedback.message}
              <Link href={feedback.action.href} className="ml-2 underline">
                {feedback.action.label}
              </Link>
            </span>
          ) : (
            feedback.message
          )}
          <button type="button" onClick={() => setFeedback(null)} className="ml-4 underline">
            Скрыть
          </button>
        </div>
      )}

      {selectedItems.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-md border bg-muted/50 p-3">
          <span className="text-sm">Выбрано: {selectedItems.length}</span>
          {status === 'rejected' ? (
            <>
              <Button size="sm" variant="outline" disabled={pending} onClick={() => handleBulkStatus('pending')}>
                Вернуть на модерацию ({selectedItems.length})
              </Button>
              <Button size="sm" disabled={pending} onClick={() => handleBulkStatus('published')}>
                Опубликовать выбранные ({selectedItems.length})
              </Button>
            </>
          ) : (
            <>
              {status === 'pending' && (
                <Button size="sm" disabled={pending} onClick={() => handleBulkStatus('published')}>
                  Опубликовать выбранные ({selectedItems.length})
                </Button>
              )}
              <Button
                size="sm"
                variant="destructive"
                disabled={pending}
                onClick={() => handleBulkStatus('rejected')}
              >
                Отклонить выбранные ({selectedItems.length})
              </Button>
            </>
          )}
          {renderBulkActions?.(selectedItems, controls)}
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">{copy.loadError}</div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : items.length === 0 ? (
        !error && (
          <div className="rounded-lg border p-8 text-center text-muted-foreground">{emptyText}</div>
        )
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-8">
                    <input
                      type="checkbox"
                      className="h-4 w-4"
                      aria-label="Выбрать все на странице"
                      checked={allSelected}
                      onChange={toggleAll}
                    />
                  </TableHead>
                  {columns.map((column) => (
                    <TableHead key={column.id} className={column.headClassName}>
                      {column.header}
                    </TableHead>
                  ))}
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => {
                  const id = getId(item);
                  return (
                    <QueueRow
                      key={id}
                      item={item}
                      name={getName(item)}
                      status={getStatus(item)}
                      columns={columns}
                      selected={selected.has(id)}
                      ctx={{
                        ...controls,
                        statusPending: pending,
                        changeStatus: (next) => handleRowStatus(item, next),
                      }}
                      onToggle={() => toggleRow(id)}
                      renderRowActions={renderRowActions}
                    />
                  );
                })}
              </TableBody>
            </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => updateParams({ page: String(p) })}
          />
        </>
      )}

      {renderOverlay?.(controls)}
    </div>
  );
}

interface QueueRowProps<TItem> {
  item: TItem;
  name: string;
  status: ModerationStatus;
  columns: ModerationQueueColumn<TItem>[];
  selected: boolean;
  ctx: ModerationRowContext;
  onToggle: () => void;
  renderRowActions?: (item: TItem, ctx: ModerationRowContext) => ReactNode;
}

function QueueRow<TItem>({
  item,
  name,
  status,
  columns,
  selected,
  ctx,
  onToggle,
  renderRowActions,
}: QueueRowProps<TItem>) {
  const pending = ctx.statusPending;
  const onStatus = ctx.changeStatus;

  return (
    <TableRow>
      <TableCell className="align-top">
        <input
          type="checkbox"
          className="h-4 w-4"
          aria-label={`Выбрать «${name}»`}
          checked={selected}
          onChange={onToggle}
        />
      </TableCell>
      {columns.map((column) => (
        <TableCell key={column.id} className={column.cellClassName}>
          {column.cell(item, ctx)}
        </TableCell>
      ))}
      <TableCell className="align-top text-right">
        <div className="flex flex-col items-end gap-1">
          {status === 'rejected' ? (
            <>
              <Button variant="outline" size="sm" disabled={pending} onClick={() => onStatus('pending')}>
                Вернуть на модерацию
              </Button>
              <Button size="sm" disabled={pending} onClick={() => onStatus('published')}>
                Опубликовать
              </Button>
            </>
          ) : (
            <>
              {status === 'pending' && (
                <Button size="sm" disabled={pending} onClick={() => onStatus('published')}>
                  Опубликовать
                </Button>
              )}
              <Button
                variant="destructive"
                size="sm"
                disabled={pending}
                onClick={() => onStatus('rejected')}
              >
                Отклонить
              </Button>
            </>
          )}
          {renderRowActions?.(item, ctx)}
        </div>
      </TableCell>
    </TableRow>
  );
}
