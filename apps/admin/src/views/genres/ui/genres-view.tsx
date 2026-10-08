'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminGenresApi, GenreStatusBadge, GENRE_STATUS_LABELS } from '@/entities/genre';
import { AdminGenre, AdminGenreInput, AdminGenreStatus, ApiError } from '@bookhaven/api-client';
import {
  Input,
  Button,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Search } from 'lucide-react';
import { Pagination } from '@/shared/ui';
import { GenreForm } from '@/features/manage-genre';

const STATUS_FILTER_OPTIONS = [
  { value: '', label: 'Все статусы' },
  { value: 'pending', label: GENRE_STATUS_LABELS.pending },
  { value: 'published', label: GENRE_STATUS_LABELS.published },
  { value: 'rejected', label: GENRE_STATUS_LABELS.rejected },
];

function parseStatus(value: string | null): AdminGenreStatus | undefined {
  return value === 'pending' || value === 'published' || value === 'rejected' ? value : undefined;
}

export function GenresView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [editing, setEditing] = useState<AdminGenre | null>(null);
  const [formData, setFormData] = useState<AdminGenreInput>({ name: '' });
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const status = parseStatus(searchParams.get('status'));
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'genres', { search, page, status }],
    queryFn: () =>
      adminGenresApi.list({
        search: search || undefined,
        page,
        limit,
        status,
      }),
  });

  const createMutation = useMutation({
    mutationFn: adminGenresApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      setFormData({ name: '' });
      // Manually created genres are published right away.
      setSuccessMessage('Жанр создан');
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: AdminGenreInput }) =>
      adminGenresApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      setEditing(null);
      setFormData({ name: '' });
      setSuccessMessage('Жанр обновлен');
    },
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSuccessMessage(null);
    if (editing) {
      updateMutation.mutate({ id: editing.id, data: formData });
    } else {
      createMutation.mutate(formData);
    }
  };

  const startEdit = (genre: AdminGenre) => {
    setSuccessMessage(null);
    setEditing(genre);
    setFormData({
      name: genre.name,
      slug: genre.slug,
      status: genre.status,
      wikidata_qid: genre.wikidata_qid ?? '',
    });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  // A taken QID may belong to a merged (hidden) genre: its QID stays reserved
  // (docs/features/genre-moderation-queue.md rule 6).
  const mutationError = createMutation.error || updateMutation.error;
  const mutationErrorText =
    mutationError instanceof ApiError && mutationError.status === 409 && /qid/i.test(mutationError.message)
      ? `${mutationError.message} (возможно, у слитого жанра)`
      : mutationError?.message;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-3xl font-bold tracking-tight">Жанры</h1>
        <Link href={'/moderation/genres' as any} className="text-sm text-primary hover:underline">
          Очередь модерации
        </Link>
      </div>

      <GenreForm
        formData={formData}
        onFieldChange={(patch) => setFormData((prev) => ({ ...prev, ...patch }))}
        editing={editing}
        onSubmit={handleSubmit}
        onCancel={() => {
          setEditing(null);
          setFormData({ name: '' });
          setSuccessMessage(null);
        }}
        pending={createMutation.isPending || updateMutation.isPending}
      />

      {successMessage && (
        <div className="rounded-md bg-green-100 p-3 text-sm text-green-900">
          {successMessage}
          <button
            type="button"
            onClick={() => setSuccessMessage(null)}
            className="ml-4 underline"
          >
            Скрыть
          </button>
        </div>
      )}
      {mutationError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {mutationErrorText}
        </div>
      )}

      <form onSubmit={handleSearch} className="flex gap-2">
        <Input
          placeholder="Поиск по названию"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button type="submit" variant="outline">
          <Search className="h-4 w-4" />
        </Button>
        <Select
          aria-label="Статус"
          options={STATUS_FILTER_OPTIONS}
          value={status ?? ''}
          onChange={(e) => updateParams({ status: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
      </form>

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить жанры.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : data?.items.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-muted-foreground">
          Жанры не найдены.
        </div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Название</TableHead>
                  <TableHead>Slug</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead>QID</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((genre: AdminGenre) => (
                  <TableRow key={genre.id}>
                    <TableCell className="font-medium">{genre.name}</TableCell>
                    <TableCell>{genre.slug}</TableCell>
                    <TableCell>
                      <GenreStatusBadge status={genre.status} />
                    </TableCell>
                    <TableCell>
                      {genre.wikidata_qid ? (
                        <a
                          href={`https://www.wikidata.org/wiki/${genre.wikidata_qid}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-primary underline-offset-4 hover:underline"
                        >
                          {genre.wikidata_qid}
                        </a>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => startEdit(genre)}>
                        Редактировать
                      </Button>
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
      )}
    </div>
  );
}
