'use client';

import { useState } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { Search } from 'lucide-react';
import { adminBooksApi } from '@/entities/book';
import {
  AdminBook,
  AdminBookBulkStatusReport,
  AdminBookStatus,
  ApiError,
} from '@bookhaven/api-client';
import {
  Button,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Pagination } from '@/shared/ui';

type StatusAction = { ids: string[]; status: Extract<AdminBookStatus, 'published' | 'rejected'> };

/**
 * Book moderation queue: every book with status = pending, whatever its
 * source (docs/features/book-status-model.md). Row actions and the
 * "publish this page" header button all go through bulkStatus; the bulk
 * scope is the displayed page (20 rows, under the server cap of 100), not
 * the search.
 */
export function BookModerationView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [feedback, setFeedback] = useState<{
    type: 'success' | 'error';
    message: string;
  } | null>(null);
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const activeSearch = searchParams.get('search') || '';
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'books', 'pending', { search: activeSearch, page }],
    queryFn: () =>
      adminBooksApi.list({
        status: 'pending',
        search: activeSearch || undefined,
        page,
        limit,
      }),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
    queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
  };

  const statusMutation = useMutation({
    mutationFn: ({ ids, status }: StatusAction) => adminBooksApi.bulkStatus(ids, status),
    onSuccess: (report: AdminBookBulkStatusReport, { status }: StatusAction) => {
      invalidate();
      const verb = status === 'published' ? 'Опубликовано' : 'Отклонено';
      const summary = `${verb} ${report.updated} из ${report.processed}.`;
      if (report.errors.length > 0) {
        const sample = report.errors
          .slice(0, 3)
          .map((e) => `«${e.title || e.book_id}»: ${e.message}`)
          .join('; ');
        setFeedback({
          type: 'error',
          message: `${summary} Ошибок: ${report.errors.length} (${sample}).`,
        });
      } else {
        setFeedback({ type: 'success', message: summary });
      }
    },
    onError: (error) => {
      const message =
        error instanceof ApiError
          ? error.message || 'Не удалось изменить статус.'
          : 'Не удалось изменить статус.';
      setFeedback({ type: 'error', message });
    },
  });

  const handleRowAction = (book: AdminBook, status: StatusAction['status']) => {
    setFeedback(null);
    const question =
      status === 'published'
        ? `Опубликовать книгу «${book.title}»?`
        : `Отклонить книгу «${book.title}»?`;
    if (!confirm(question)) {
      return;
    }
    statusMutation.mutate({ ids: [book.id], status });
  };

  const pageIds = data?.items.map((b) => b.id) ?? [];

  const handlePublishPage = () => {
    setFeedback(null);
    if (!confirm(`Опубликовать все книги на этой странице (${pageIds.length})?`)) {
      return;
    }
    statusMutation.mutate({ ids: pageIds, status: 'published' });
  };

  const updateParams = (updates: Record<string, string | undefined>) => {
    const params = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        params.delete(key);
      } else {
        params.set(key, value);
      }
    });
    router.push(`${pathname}?${params.toString()}` as any);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    updateParams({ search, page: '1' });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Книги на проверке</h1>
        {pageIds.length > 0 && (
          <Button onClick={handlePublishPage} disabled={statusMutation.isPending}>
            Опубликовать все на странице ({pageIds.length})
          </Button>
        )}
      </div>

      <form onSubmit={handleSearch} className="flex gap-2">
        <Input
          placeholder="Поиск по названию, ISBN, описанию"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button type="submit" variant="outline" aria-label="Найти">
          <Search className="h-4 w-4" />
        </Button>
      </form>

      {feedback && (
        <div
          className={`rounded-md p-4 text-sm ${
            feedback.type === 'success'
              ? 'bg-green-100 text-green-900'
              : 'bg-destructive/10 text-destructive'
          }`}
        >
          {feedback.message}
          <button onClick={() => setFeedback(null)} className="ml-4 underline" type="button">
            Скрыть
          </button>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить книги.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : data?.items.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-muted-foreground">
          Нет книг на проверке.
        </div>
      ) : data ? (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Название</TableHead>
                  <TableHead>Авторы</TableHead>
                  <TableHead>Жанры</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((book: AdminBook) => (
                  <TableRow key={book.id}>
                    <TableCell className="font-medium">
                      {book.title}
                      {book.external_source && (
                        <div className="text-xs font-normal text-muted-foreground">
                          {book.external_source}
                        </div>
                      )}
                    </TableCell>
                    <TableCell>{book.authors.map((a) => a.name).join(', ') || '—'}</TableCell>
                    <TableCell>{book.genres.map((g) => g.name).join(', ') || '—'}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleRowAction(book, 'published')}
                          disabled={statusMutation.isPending}
                        >
                          Опубликовать
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleRowAction(book, 'rejected')}
                          disabled={statusMutation.isPending}
                        >
                          Отклонить
                        </Button>
                        <Link href={`/catalog/books/${book.id}/edit` as any}>
                          <Button variant="ghost" size="sm">
                            Редактировать
                          </Button>
                        </Link>
                      </div>
                    </TableCell>
                  </TableRow>
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
      ) : null}
    </div>
  );
}
