'use client';

import { useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  AdminAuthor,
  AdminAuthorInput,
} from '@bookhaven/api-client';
import {
  Button,
  Input,
  Label,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Search } from 'lucide-react';
import { Pagination } from '@/components/pagination';

export default function AuthorsPage() {
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
      api.admin.authors.list({
        search: search || undefined,
        page,
        limit,
      }),
  });

  const createMutation = useMutation({
    mutationFn: api.admin.authors.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setFormData({ name: '' });
      setSuccessMessage('Автор создан');
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: AdminAuthorInput }) =>
      api.admin.authors.update(id, data),
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

      <form onSubmit={handleSubmit} className="space-y-4 rounded-lg border p-4">
        <h2 className="font-semibold">
          {editing ? 'Редактировать автора' : 'Новый автор'}
        </h2>
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="name">Имя *</Label>
            <Input
              id="name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="slug">Slug</Label>
            <Input
              id="slug"
              value={formData.slug || ''}
              onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
              placeholder="Автоматически из имени"
            />
          </div>
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="photo_url">URL фото</Label>
            <Input
              id="photo_url"
              value={formData.photo_url || ''}
              onChange={(e) => setFormData({ ...formData, photo_url: e.target.value })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="birth_date">Дата рождения</Label>
            <Input
              id="birth_date"
              type="date"
              value={formData.birth_date || ''}
              onChange={(e) => setFormData({ ...formData, birth_date: e.target.value })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="death_date">Дата смерти</Label>
            <Input
              id="death_date"
              type="date"
              value={formData.death_date || ''}
              onChange={(e) => setFormData({ ...formData, death_date: e.target.value })}
            />
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="biography">Биография</Label>
          <Input
            id="biography"
            value={formData.biography || ''}
            onChange={(e) => setFormData({ ...formData, biography: e.target.value })}
          />
        </div>
        <div className="flex gap-2">
          <Button type="submit" disabled={createMutation.isPending || updateMutation.isPending}>
            {editing ? 'Сохранить' : 'Создать'}
          </Button>
          {editing && (
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setEditing(null);
                setFormData({ name: '' });
                setSuccessMessage(null);
              }}
            >
              Отмена
            </Button>
          )}
        </div>
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
      </form>

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
