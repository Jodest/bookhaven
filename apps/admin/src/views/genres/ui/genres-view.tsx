'use client';

import { useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminGenresApi } from '@/entities/genre';
import { AdminGenre, AdminGenreInput } from '@bookhaven/api-client';
import {
  Input,
  Button,
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
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'genres', { search, page }],
    queryFn: () =>
      adminGenresApi.list({
        search: search || undefined,
        page,
        limit,
      }),
  });

  const createMutation = useMutation({
    mutationFn: adminGenresApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      setFormData({ name: '' });
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
    setFormData({ name: genre.name, slug: genre.slug });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Жанры</h1>

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
      {(createMutation.error || updateMutation.error) && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {createMutation.error?.message || updateMutation.error?.message}
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
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((genre: AdminGenre) => (
                  <TableRow key={genre.id}>
                    <TableCell className="font-medium">{genre.name}</TableCell>
                    <TableCell>{genre.slug}</TableCell>
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
