'use client';

import { useState } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { adminBooksApi } from '@/entities/book';
import { AdminBook } from '@bookhaven/api-client';
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

  const [search, setSearch] = useState(searchParams.get('search') || '');
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const visibility = searchParams.get('visibility') || '';
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'books', { search, visibility, page }],
    queryFn: () =>
      adminBooksApi.list({
        search: search || undefined,
        visibility: visibility as 'visible' | 'hidden' | '',
        page,
        limit,
      }),
  });

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
        <Link href={"/books/new" as any}>
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
          options={[
            { value: '', label: 'Все' },
            { value: 'visible', label: 'Видимые' },
            { value: 'hidden', label: 'Скрытые' },
          ]}
          value={visibility}
          onChange={(e) => updateParams({ visibility: e.target.value, page: '1' })}
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
                      {book.is_hidden ? (
                        <Badge variant="secondary">Скрыта</Badge>
                      ) : (
                        <Badge variant="default">Видима</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Link href={`/books/${book.id}/edit` as any}>
                        <Button variant="ghost" size="sm">
                          Редактировать
                        </Button>
                      </Link>
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
