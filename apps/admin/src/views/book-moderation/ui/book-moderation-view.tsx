'use client';

import { useState } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { Search } from 'lucide-react';
import { adminBooksApi } from '@/entities/book';
import { AdminBook, ApiError, BulkPublishReport } from '@bookhaven/api-client';
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

/**
 * Pending-review queue: hidden books imported from an external source.
 * Search-only on purpose — `bulkPublish` honours only `search`, so the bulk
 * scope always equals the displayed list.
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
    queryKey: ['admin', 'books', 'pending_review', { search: activeSearch, page }],
    queryFn: () =>
      adminBooksApi.list({
        visibility: 'pending_review',
        search: activeSearch || undefined,
        page,
        limit,
      }),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
    queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
  };

  const publishMutation = useMutation({
    mutationFn: (book: AdminBook) =>
      adminBooksApi.update(book.id, {
        title: book.title,
        slug: book.slug,
        description: book.description,
        cover_url: book.cover_url,
        isbn: book.isbn,
        published_year: book.published_year,
        page_count: book.page_count,
        language: book.language,
        author_ids: book.authors.map((a) => a.id),
        genre_ids: book.genres.map((g) => g.id),
        is_hidden: false,
      }),
    onSuccess: () => {
      invalidate();
      setFeedback({ type: 'success', message: 'Книга опубликована.' });
    },
    onError: (error) => {
      const message =
        error instanceof ApiError
          ? error.message || 'Не удалось опубликовать книгу.'
          : 'Не удалось опубликовать книгу.';
      setFeedback({ type: 'error', message });
    },
  });

  const handlePublish = (book: AdminBook) => {
    setFeedback(null);
    if (!confirm(`Опубликовать книгу «${book.title}»?`)) {
      return;
    }
    publishMutation.mutate(book);
  };

  const bulkPublishMutation = useMutation({
    mutationFn: (params: { search?: string }) => adminBooksApi.bulkPublish(params),
    onSuccess: (report: BulkPublishReport) => {
      invalidate();
      const summary = `Опубликовано ${report.published} из ${report.processed}.`;
      if (report.errors.length > 0) {
        const sample = report.errors
          .slice(0, 3)
          .map((e) => `«${e.title}»: ${e.message}`)
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
          ? error.message || 'Не удалось опубликовать книги.'
          : 'Не удалось опубликовать книги.';
      setFeedback({ type: 'error', message });
    },
  });

  const total = data?.total ?? 0;

  const handleBulkPublish = () => {
    setFeedback(null);
    if (!confirm(`Опубликовать все книги на проверке (${total})?`)) {
      return;
    }
    bulkPublishMutation.mutate({ search: activeSearch || undefined });
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
        {total > 0 && (
          <Button onClick={handleBulkPublish} disabled={bulkPublishMutation.isPending}>
            Опубликовать все ({total})
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
                          onClick={() => handlePublish(book)}
                          disabled={publishMutation.isPending}
                        >
                          Опубликовать
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
