'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import {
  AdminGenre,
  AdminGenreBulkStatusReport,
  AdminGenreHint,
  AdminGenreQueueItem,
  AdminGenreQueueParams,
  AdminGenreStatus,
  ApiError,
} from '@bookhaven/api-client';
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
import { adminGenresApi, GenreStatusBadge } from '@/entities/genre';
import { BOOK_STATUS_LABELS } from '@/entities/book';
import { MergeGenreDialog } from '@/features/merge-genre';
import {
  bulkGenreStatusConfirmText,
  bulkGenreStatusSummary,
  genreStatusConfirmText,
  useGenreStatusMutation,
} from '@/features/moderate-genre';
import { Pagination } from '@/shared/ui';

// Genre moderation queue (docs/features/genre-moderation-queue.md).

const LIMIT = 20;

const STATUS_TABS: { value: AdminGenreStatus; label: string }[] = [
  { value: 'pending', label: 'На модерации' },
  { value: 'published', label: 'Опубликованные' },
  { value: 'rejected', label: 'Отклонённые' },
];

const QID_OPTIONS = [
  { value: '', label: 'Все' },
  { value: 'with', label: 'С QID' },
  { value: 'without', label: 'Без QID' },
];

const MATCH_LABELS: Record<AdminGenreHint['match'], string> = {
  exact_name: 'то же название',
  same_stem: 'та же основа',
  shared_word: 'общее слово',
};

const SUCCESS_MESSAGES: Record<AdminGenreStatus, (name: string) => string> = {
  published: (name) => `Жанр «${name}» опубликован.`,
  rejected: (name) => `Жанр «${name}» отклонён.`,
  pending: (name) => `Жанр «${name}» возвращён на модерацию.`,
};

function parseStatus(value: string | null): AdminGenreStatus {
  return value === 'published' || value === 'rejected' ? value : 'pending';
}

function parseQid(value: string | null): AdminGenreQueueParams['qid'] {
  return value === 'with' || value === 'without' ? value : undefined;
}

function WikidataLink({ qid }: { qid: string | null }) {
  if (!qid) return <span className="text-muted-foreground">—</span>;
  return (
    <a
      href={`https://www.wikidata.org/wiki/${qid}`}
      target="_blank"
      rel="noopener noreferrer"
      className="font-mono text-xs text-primary underline"
    >
      {qid}
    </a>
  );
}

type Feedback = { type: 'success' | 'error'; message: string };

interface MergeTarget {
  source: AdminGenre;
  initialCanonical?: AdminGenre;
}

export function GenreModerationView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const status = parseStatus(searchParams.get('status'));
  const qid = parseQid(searchParams.get('qid'));
  const search = searchParams.get('search') || '';
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1);

  const [searchInput, setSearchInput] = useState(search);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [mergeTarget, setMergeTarget] = useState<MergeTarget | null>(null);

  const paramsKey = searchParams.toString();
  // Selection belongs to the current page and filters only.
  useEffect(() => {
    setSelected(new Set());
  }, [paramsKey]);

  const params: AdminGenreQueueParams = {
    status,
    qid,
    search: search || undefined,
    page,
    limit: LIMIT,
  };

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'genres', 'moderation', params],
    queryFn: () => adminGenresApi.moderationQueue(params),
  });

  const statusMutation = useGenreStatusMutation();

  const items = data?.items ?? [];
  const totalPages = data ? Math.ceil(data.total / LIMIT) : 0;

  const updateParams = (updates: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });
    setSelected(new Set());
    router.push(`${pathname}?${next.toString()}` as any);
  };

  const errorText = (err: unknown) =>
    err instanceof ApiError && err.message ? err.message : 'Не удалось изменить статус жанра.';

  const handleRowStatus = (item: AdminGenreQueueItem, next: AdminGenreStatus) => {
    setFeedback(null);
    if (!confirm(genreStatusConfirmText(next, item.genre.name, item.books_visible))) {
      return;
    }
    statusMutation.mutate(
      { ids: [item.genre.id], status: next },
      {
        onSuccess: (report: AdminGenreBulkStatusReport) => {
          if (report.errors.length > 0) {
            setFeedback({
              type: 'error',
              message: `Не удалось изменить статус «${item.genre.name}»: ${report.errors[0].message}.`,
            });
            return;
          }
          setFeedback({ type: 'success', message: SUCCESS_MESSAGES[next](item.genre.name) });
        },
        onError: (err) => setFeedback({ type: 'error', message: errorText(err) }),
      }
    );
  };

  const selectedItems = items.filter((i) => selected.has(i.genre.id));

  const handleBulkStatus = (next: AdminGenreStatus) => {
    setFeedback(null);
    if (selectedItems.length === 0) return;
    const visible = selectedItems.reduce((sum, i) => sum + i.books_visible, 0);
    if (!confirm(bulkGenreStatusConfirmText(next, selectedItems.length, visible))) {
      return;
    }
    const names = Object.fromEntries(selectedItems.map((i) => [i.genre.id, i.genre.name]));
    statusMutation.mutate(
      { ids: selectedItems.map((i) => i.genre.id), status: next },
      {
        onSuccess: (report: AdminGenreBulkStatusReport) => {
          setSelected(new Set());
          setFeedback({
            type: report.errors.length > 0 ? 'error' : 'success',
            message: bulkGenreStatusSummary(report, names),
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

  const allSelected = items.length > 0 && items.every((i) => selected.has(i.genre.id));
  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(items.map((i) => i.genre.id)));
  };

  const openMerge = (source: AdminGenre, initialCanonical?: AdminGenre) => {
    setFeedback(null);
    setMergeTarget({ source, initialCanonical });
  };

  const hasExtraFilters = Boolean(qid || search);
  const emptyText =
    status === 'pending' && !hasExtraFilters ? 'Нет жанров на модерации' : 'Жанры не найдены';
  const pending = statusMutation.isPending;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Модерация жанров</h1>

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
          placeholder="Поиск по названию"
          aria-label="Поиск"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
        <Button type="submit" variant="outline" aria-label="Найти">
          <Search className="h-4 w-4" />
        </Button>
        <Select
          aria-label="QID"
          options={QID_OPTIONS}
          value={qid ?? ''}
          onChange={(e) => updateParams({ qid: e.target.value, page: undefined })}
          className="w-full md:w-40"
        />
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
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить очередь жанров.
        </div>
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
                  <TableHead>Жанр</TableHead>
                  <TableHead>QID</TableHead>
                  <TableHead>Добавлен</TableHead>
                  <TableHead>Книги</TableHead>
                  <TableHead>Возможно, тот же жанр</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <QueueRow
                    key={item.genre.id}
                    item={item}
                    selected={selected.has(item.genre.id)}
                    pending={pending}
                    onToggle={() => toggleRow(item.genre.id)}
                    onStatus={(next) => handleRowStatus(item, next)}
                    onMerge={(canonical) => openMerge(item.genre, canonical)}
                  />
                ))}
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

      {mergeTarget && (
        <MergeGenreDialog
          source={mergeTarget.source}
          initialCanonical={mergeTarget.initialCanonical}
          onClose={() => setMergeTarget(null)}
          onMerged={(booksReassigned) => {
            const name = mergeTarget.source.name;
            setMergeTarget(null);
            setSelected(new Set());
            setFeedback({
              type: 'success',
              message: `Жанр «${name}» слит. Перенесено книг: ${booksReassigned}.`,
            });
          }}
        />
      )}
    </div>
  );
}

interface QueueRowProps {
  item: AdminGenreQueueItem;
  selected: boolean;
  pending: boolean;
  onToggle: () => void;
  onStatus: (status: AdminGenreStatus) => void;
  onMerge: (canonical?: AdminGenre) => void;
}

function QueueRow({ item, selected, pending, onToggle, onStatus, onMerge }: QueueRowProps) {
  const { genre } = item;
  const ownQid = genre.wikidata_qid;

  return (
    <TableRow>
      <TableCell className="align-top">
        <input
          type="checkbox"
          className="h-4 w-4"
          aria-label={`Выбрать «${genre.name}»`}
          checked={selected}
          onChange={onToggle}
        />
      </TableCell>
      <TableCell className="align-top">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium">{genre.name}</span>
          <GenreStatusBadge status={genre.status} />
        </div>
        <div className="text-xs text-muted-foreground">{genre.slug}</div>
      </TableCell>
      <TableCell className="align-top">
        <WikidataLink qid={ownQid} />
      </TableCell>
      <TableCell className="align-top whitespace-nowrap text-sm">
        {new Date(item.created_at).toLocaleDateString('ru-RU')}
      </TableCell>
      <TableCell className="align-top text-sm">
        <div>
          {item.books_total} (видимых {item.books_visible})
        </div>
        {item.sample_books.length > 0 && (
          <ul className="mt-1 space-y-0.5">
            {item.sample_books.map((b) => (
              <li key={b.id}>
                <Link href={`/catalog/books/${b.id}/edit` as any} className="text-primary underline">
                  {b.title}
                </Link>
                {b.status !== 'published' && (
                  <span className="ml-1 text-xs text-muted-foreground">
                    ({(BOOK_STATUS_LABELS[b.status] ?? b.status).toLowerCase()})
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </TableCell>
      <TableCell className="align-top text-sm">
        {item.similar.length === 0 ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          <ul className="space-y-2">
            {item.similar.map((hint) => {
              const differentQid =
                !!ownQid && !!hint.genre.wikidata_qid && hint.genre.wikidata_qid !== ownQid;
              return (
                <li key={hint.genre.id} className="space-y-1">
                  <div className="flex flex-wrap items-center gap-1">
                    <span className="font-medium">{hint.genre.name}</span>
                    <span className="text-xs text-muted-foreground">({MATCH_LABELS[hint.match]})</span>
                    <GenreStatusBadge status={hint.genre.status} />
                    {hint.genre.wikidata_qid && <WikidataLink qid={hint.genre.wikidata_qid} />}
                  </div>
                  {differentQid && (
                    <div className="text-xs text-amber-700">разные QID — возможно, разные понятия</div>
                  )}
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={pending}
                    onClick={() => onMerge(hint.genre)}
                  >
                    Это он — слить
                  </Button>
                </li>
              );
            })}
          </ul>
        )}
      </TableCell>
      <TableCell className="align-top text-right">
        <div className="flex flex-col items-end gap-1">
          {genre.status === 'rejected' ? (
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
              {genre.status === 'pending' && (
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
          <Button variant="ghost" size="sm" disabled={pending} onClick={() => onMerge()}>
            Слить
          </Button>
        </div>
      </TableCell>
    </TableRow>
  );
}
