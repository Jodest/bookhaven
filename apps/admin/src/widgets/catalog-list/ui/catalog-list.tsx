'use client';

import { useEffect, useState } from 'react';
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
import { buildEditHref, consumeFlash } from '@/shared/lib';
import { Pagination } from '@/shared/ui';
import { useCatalogUrlState } from '../model/use-catalog-url-state';
import type { CatalogFeedback, CatalogListFilter, CatalogListProps, CatalogRowContext } from '../model/types';

// Shared catalog list shell (docs/features/catalog-unified-lists.md): URL
// state, search + status + entity filters, feedback banner (incl. the
// one-shot flash from an edit page), «Редактировать» link carrying `back`,
// «Снять с публикации» (→ pending) with confirm, states, table, pagination.

const DEFAULT_LIMIT = 20;
const NO_FILTERS: CatalogListFilter[] = [];

export function CatalogList<TItem, TParams, TReport extends { errors: { message: string }[] }>({
  title,
  headerExtra,
  entityPath,
  flashKey,
  queryKey,
  buildParams,
  queryFn,
  statusMutation,
  getId,
  getName,
  getStatus,
  statusLabels,
  filters = NO_FILTERS,
  columns,
  renderRowActions,
  copy,
  limit = DEFAULT_LIMIT,
}: CatalogListProps<TItem, TParams, TReport>) {
  const { state, search, pathname, searchParams, pushParams } = useCatalogUrlState(filters, limit);
  const { page } = state;

  const [searchInput, setSearchInput] = useState(search);
  const [feedback, setFeedback] = useState<CatalogFeedback | null>(null);

  // Re-sync the input when the URL changes (incl. back navigation).
  useEffect(() => {
    setSearchInput(search);
  }, [search]);

  // One-shot success message left by an edit page.
  useEffect(() => {
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

  const statusOptions = [
    { value: '', label: 'Все статусы' },
    { value: 'published', label: statusLabels.published },
    { value: 'pending', label: statusLabels.pending },
    { value: 'rejected', label: statusLabels.rejected },
  ];

  const errorText = (err: unknown) =>
    err instanceof ApiError && err.message ? err.message : copy.statusErrorFallback;

  const handleUnpublish = (item: TItem) => {
    setFeedback(null);
    const name = getName(item);
    if (!confirm(copy.unpublishConfirm(name))) {
      return;
    }
    statusMutation.mutate(
      { ids: [getId(item)], status: 'pending' },
      {
        onSuccess: (report) => {
          if (report.errors.length > 0) {
            setFeedback({
              type: 'error',
              message: `Не удалось снять с публикации «${name}»: ${report.errors[0].message}.`,
            });
            return;
          }
          setFeedback({ type: 'success', message: copy.unpublishSuccess(name) });
        },
        onError: (err) => setFeedback({ type: 'error', message: errorText(err) }),
      }
    );
  };

  const ctx: CatalogRowContext = { notify: setFeedback };
  const pending = statusMutation.isPending;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
        {headerExtra}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          pushParams({ search: searchInput.trim(), page: undefined });
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
        <Select
          aria-label="Статус"
          options={statusOptions}
          value={state.status ?? ''}
          onChange={(e) => pushParams({ status: e.target.value, page: undefined })}
          className="w-full md:w-48"
        />
        {filters.map((filter) => (
          <Select
            key={filter.param}
            aria-label={filter.label}
            options={filter.options}
            value={state.filters[filter.param] ?? ''}
            onChange={(e) => pushParams({ [filter.param]: e.target.value, page: undefined })}
            className="w-full md:w-48"
          />
        ))}
      </form>

      {feedback && (
        <div
          className={`flex items-center justify-between rounded-md p-3 text-sm ${
            feedback.type === 'success'
              ? 'bg-green-100 text-green-900'
              : 'bg-destructive/10 text-destructive'
          }`}
        >
          {feedback.message}
          <button type="button" onClick={() => setFeedback(null)} className="ml-4 underline">
            Скрыть
          </button>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">{copy.loadError}</div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="catalog-list-loading" />
      ) : items.length === 0 ? (
        !error && (
          <div className="rounded-lg border p-8 text-center text-muted-foreground">{copy.empty}</div>
        )
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
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
                  const editHref = buildEditHref(
                    `${entityPath}/${encodeURIComponent(id)}/edit`,
                    pathname,
                    searchParams
                  );
                  return (
                    <TableRow key={id}>
                      {columns.map((column) => (
                        <TableCell key={column.id} className={column.cellClassName}>
                          {column.cell(item)}
                        </TableCell>
                      ))}
                      <TableCell className="text-right">
                        <div className="flex flex-wrap justify-end gap-2">
                          <Button asChild variant="ghost" size="sm">
                            <Link href={editHref as any}>Редактировать</Link>
                          </Button>
                          {getStatus(item) === 'published' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-destructive"
                              disabled={pending}
                              onClick={() => handleUnpublish(item)}
                            >
                              Снять с публикации
                            </Button>
                          )}
                          {renderRowActions?.(item, ctx)}
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => pushParams({ page: String(p) })}
          />
        </>
      )}
    </div>
  );
}
