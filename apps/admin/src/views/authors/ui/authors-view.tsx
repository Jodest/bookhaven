'use client';

import { useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAuthorsApi } from '@/entities/author';
import { AdminAuthor, AdminAuthorInput } from '@bookhaven/api-client';
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
import { AuthorForm } from '@/features/manage-author';

export function AuthorsView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [editing, setEditing] = useState<AdminAuthor | null>(null);
  const [formData, setFormData] = useState<AdminAuthorInput>({ name: '' });
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'authors', { search, page }],
    queryFn: () =>
      adminAuthorsApi.list({
        search: search || undefined,
        page,
        limit,
      }),
  });

  const createMutation = useMutation({
    mutationFn: adminAuthorsApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setFormData({ name: '' });
      setSuccessMessage('Автор создан');
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: AdminAuthorInput }) =>
      adminAuthorsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setEditing(null);
      setFormData({ name: '' });
      setSuccessMessage('Автор обновлен');
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

  const startEdit = (author: AdminAuthor) => {
    setSuccessMessage(null);
    setEditing(author);
    setFormData({
      name: author.name,
      slug: author.slug,
      biography: author.biography ?? '',
      photo_url: author.photo_url ?? '',
      birth_date: author.birth_date ?? '',
      death_date: author.death_date ?? '',
    });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Авторы</h1>

      <AuthorForm
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
        successMessage={successMessage}
        onDismissSuccess={() => setSuccessMessage(null)}
        errorMessage={createMutation.error?.message || updateMutation.error?.message}
      />

      <form onSubmit={handleSearch} className="flex gap-2">
        <Input
          placeholder="Поиск по имени"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button type="submit" variant="outline">
          <Search className="h-4 w-4" />
        </Button>
      </form>

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить авторов.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : data?.items.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-muted-foreground">
          Авторы не найдены.
        </div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Имя</TableHead>
                  <TableHead>Slug</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((author: AdminAuthor) => (
                  <TableRow key={author.id}>
                    <TableCell className="font-medium">{author.name}</TableCell>
                    <TableCell>{author.slug}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => startEdit(author)}>
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
