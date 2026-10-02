'use client';

import { useState } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminBooksApi } from '@/entities/book';
import { adminAuthorsApi } from '@/entities/author';
import { adminGenresApi } from '@/entities/genre';
import { AdminBook, ApiError, BulkPublishReport } from '@bookhaven/api-client';
import {
  Button,
  Input,
  Badge,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Select,
} from '@bookhaven/ui';
import Link from 'next/link';
import { Plus, Search } from 'lucide-react';
import { Pagination } from '@/shared/ui';

export function BooksListView() {
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
  const visibility = searchParams.get('visibility') || '';
  const genreId = searchParams.get('genre_id') || '';
  const authorId = searchParams.get('author_id') || '';
  const source = searchParams.get('source') || '';
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'books', { search, visibility, genreId, authorId, source, page }],
    queryFn: () =>
      adminBooksApi.list({
        search: search || undefined,
        visibility: visibility as 'visible' | 'hidden' | 'pending_review' | '',
        genreId: genreId || undefined,
        authorId: authorId || undefined,
        source: source as 'manual' | 'harvested' | '',
        page,
        limit,
      }),
  });

  const { data: genresData } = useQuery({
    queryKey: ['admin', 'genres', 'all'],
    queryFn: () => adminGenresApi.list({ limit: 100 }),
  });

  const { data: authorsData } = useQuery({
    queryKey: ['admin', 'authors', 'all'],
    queryFn: () => adminAuthorsApi.list({ limit: 100 }),
  });

  const genreOptions = [
    { value: '', label: 'Все жанры' },
    ...(genresData?.items.map((g) => ({ value: g.id, label: g.name })) || []),
  ];

  const authorOptions = [
    { value: '', label: 'Все авторы' },
    ...(authorsData?.items.map((a) => ({ value: a.id, label: a.name })) || []),
  ];

  const sourceOptions = [
    { value: '', label: 'Все' },
    { value: 'manual', label: 'Добавлено вручную' },
    { value: 'harvested', label: 'Импортировано' },
  ];

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
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
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
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
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

  const handleBulkPublish = () => {
    setFeedback(null);
    const total = data?.total ?? 0;
    if (!confirm(`Опубликовать все книги на проверке (${total})?`)) {
      return;
    }
    bulkPublishMutation.mutate({ search: search || undefined });
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
        <h1 className="text-3xl font-bold tracking-tight">Книги</h1>
        <div className="flex gap-2">
          <Link href={"/books/import" as any}>
            <Button variant="outline">Импортировать</Button>
          </Link>
          <Link href={"/books/new" as any}>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Добавить книгу
            </Button>
          </Link>
        </div>
      </div>

      <div className="flex flex-col gap-4 md:flex-row">
        <form onSubmit={handleSearch} className="flex flex-1 gap-2">
          <Input
            placeholder="Поиск по названию, ISBN, описанию"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <Button type="submit" variant="outline">
            <Search className="h-4 w-4" />
          </Button>
        </form>
        <Select
          options={[
            { value: '', label: 'Все' },
            { value: 'visible', label: 'Видимые' },
            { value: 'hidden', label: 'Скрытые' },
            { value: 'pending_review', label: 'Импортировано (на проверке)' },
          ]}
          value={visibility}
          onChange={(e) => updateParams({ visibility: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
        <Select
          options={genreOptions}
          value={genreId}
          onChange={(e) => updateParams({ genre_id: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
        <Select
          options={authorOptions}
          value={authorId}
          onChange={(e) => updateParams({ author_id: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
        <Select
          options={sourceOptions}
          value={source}
          onChange={(e) => updateParams({ source: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
        {visibility === 'pending_review' && (data?.total ?? 0) > 0 && (
          <Button
            variant="outline"
            onClick={handleBulkPublish}
            disabled={bulkPublishMutation.isPending}
          >
            Опубликовать все ({data?.total})
          </Button>
        )}
      </div>

      {feedback && (
        <div
          className={`rounded-md p-4 text-sm ${
            feedback.type === 'success'
              ? 'bg-green-100 text-green-900'
              : 'bg-destructive/10 text-destructive'
          }`}
        >
          {feedback.message}
          <button
            onClick={() => setFeedback(null)}
            className="ml-4 underline"
            type="button"
          >
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
          Книги не найдены.
        </div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Название</TableHead>
                  <TableHead>Авторы</TableHead>
                  <TableHead>Жанры</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((book: AdminBook) => (
                  <TableRow key={book.id}>
                    <TableCell className="font-medium">{book.title}</TableCell>
                    <TableCell>
                      {book.authors.map((a) => a.name).join(', ') || '—'}
                    </TableCell>
                    <TableCell>
                      {book.genres.map((g) => g.name).join(', ') || '—'}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {book.is_hidden ? (
                          <Badge variant="secondary">Скрыта</Badge>
                        ) : (
                          <Badge variant="default">Видима</Badge>
                        )}
                        {book.external_source && (
                          <Badge variant="outline">Импортировано</Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        {book.is_hidden && book.external_source && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handlePublish(book)}
                            disabled={publishMutation.isPending}
                          >
                            Опубликовать
                          </Button>
                        )}
                        <Link href={`/books/${book.id}/edit` as any}>
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
          <Pagination page={page} totalPages={totalPages} onPageChange={(p) => updateParams({ page: String(p) })} />
        </>
      )}
    </div>
  );
}
