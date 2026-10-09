'use client';

import { useState } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { adminBooksApi, BookStatusBadge, BOOK_STATUS_LABELS } from '@/entities/book';
import { adminAuthorsApi } from '@/entities/author';
import { adminGenresApi } from '@/entities/genre';
import { AdminBook, AdminBookStatus } from '@bookhaven/api-client';
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

const STATUS_FILTER_OPTIONS = [
  { value: '', label: 'Все статусы' },
  { value: 'published', label: BOOK_STATUS_LABELS.published },
  { value: 'pending', label: BOOK_STATUS_LABELS.pending },
  { value: 'rejected', label: BOOK_STATUS_LABELS.rejected },
];

// Only valid statuses are sent; anything else (including the legacy
// `visibility` param, which is no longer read) means "all".
function parseStatus(value: string | null): AdminBookStatus | '' {
  return value === 'pending' || value === 'published' || value === 'rejected' ? value : '';
}

export function BooksListView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [search, setSearch] = useState(searchParams.get('search') || '');
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const status = parseStatus(searchParams.get('status'));
  const genreId = searchParams.get('genre_id') || '';
  const authorId = searchParams.get('author_id') || '';
  const source = searchParams.get('source') || '';
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'books', { search, status, genreId, authorId, source, page }],
    queryFn: () =>
      adminBooksApi.list({
        search: search || undefined,
        status,
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
        <Link href={'/catalog/books/new' as any}>
          <Button>
            <Plus className="mr-2 h-4 w-4" />
            Добавить книгу
          </Button>
        </Link>
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
          aria-label="Статус"
          options={STATUS_FILTER_OPTIONS}
          value={status}
          onChange={(e) => updateParams({ status: e.target.value, page: '1' })}
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
      </div>

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
                        <BookStatusBadge status={book.status} />
                        {book.external_source && (
                          <Badge variant="outline">Импортировано</Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
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
          <Pagination page={page} totalPages={totalPages} onPageChange={(p) => updateParams({ page: String(p) })} />
        </>
      )}
    </div>
  );
}
