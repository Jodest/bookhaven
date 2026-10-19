'use client';

import { useAuthStore } from '@/entities/session';
import { shelfKeys, shelvesApi } from '@/entities/shelf';
import { invalidateShelfContents, invalidateShelfMeta } from '@/features/shelve-book';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Shelf } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const createShelfSchema = z.object({
  name: z.string().trim().min(2, 'Минимум 2 символа').max(100, 'Максимум 100 символов'),
  description: z.string().trim().max(2000, 'Максимум 2000 символов').optional(),
});

type CreateShelfForm = z.infer<typeof createShelfSchema>;

const defaultTypeOrder = ['want_to_read', 'reading', 'read'];

export function ShelvesView() {
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const [actionError, setActionError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateShelfForm>({
    resolver: zodResolver(createShelfSchema),
  });

  const query = useQuery({
    queryKey: shelfKeys.list(user?.id),
    queryFn: () => shelvesApi.list().then((response) => response.items),
  });
  const shelves = query.data ?? [];
  const loading = query.isPending;
  const error =
    actionError ??
    (query.isError
      ? query.error instanceof Error
        ? query.error.message
        : 'Ошибка загрузки полок'
      : null);

  const createMutation = useMutation({
    mutationFn: (data: CreateShelfForm) =>
      shelvesApi.create({
        name: data.name,
        ...(data.description ? { description: data.description } : {}),
      }),
    onSuccess: () => invalidateShelfMeta(queryClient),
  });
  const updateMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: { name?: string; is_public?: boolean } }) =>
      shelvesApi.update(id, body),
    onSuccess: () => invalidateShelfMeta(queryClient),
  });
  const removeMutation = useMutation({
    mutationFn: (id: string) => shelvesApi.remove(id),
    onSuccess: () => invalidateShelfContents(queryClient),
  });

  const messageOf = (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback;

  const onCreate = async (data: CreateShelfForm) => {
    setActionError(null);
    try {
      await createMutation.mutateAsync(data);
      reset();
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка создания полки'));
    }
  };

  const onRename = async (id: string, name: string) => {
    setActionError(null);
    try {
      await updateMutation.mutateAsync({ id, body: { name } });
      setEditingId(null);
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка переименования полки'));
    }
  };

  const onTogglePrivacy = async (shelf: Shelf) => {
    setActionError(null);
    try {
      await updateMutation.mutateAsync({ id: shelf.id, body: { is_public: !shelf.isPublic } });
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка изменения видимости полки'));
    }
  };

  const onDelete = async (id: string) => {
    if (!confirm('Удалить полку? Книги будут убраны из неё.')) {
      return;
    }
    setActionError(null);
    try {
      await removeMutation.mutateAsync(id);
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка удаления полки'));
    }
  };

  if (loading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  const defaultShelves = shelves
    .filter((s) => defaultTypeOrder.includes(s.type))
    .sort((a, b) => defaultTypeOrder.indexOf(a.type) - defaultTypeOrder.indexOf(b.type));
  const customShelves = shelves.filter((s) => s.type === 'custom');

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">
        Мои полки
      </h1>

      {error && (
        <div className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit(onCreate)} className="mt-6 max-w-md space-y-2">
        <div className="flex gap-2">
          <div className="flex-1">
            <input
              {...register('name')}
              placeholder="Название новой полки"
              className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
            />
            {errors.name && (
              <p className="mt-1 text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Создание...' : 'Создать'}
          </Button>
        </div>
        <div>
          <textarea
            {...register('description')}
            placeholder="Описание (необязательно)"
            rows={2}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
          {errors.description && (
            <p className="mt-1 text-sm text-destructive">{errors.description.message}</p>
          )}
        </div>
      </form>

      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {defaultShelves.map((shelf) => (
          <ShelfCard key={shelf.id} shelf={shelf} />
        ))}
        {customShelves.map((shelf) => (
          <ShelfCard
            key={shelf.id}
            shelf={shelf}
            isEditing={editingId === shelf.id}
            onStartEdit={() => setEditingId(shelf.id)}
            onRename={onRename}
            onTogglePrivacy={onTogglePrivacy}
            onDelete={onDelete}
          />
        ))}
      </div>
    </main>
  );
}

interface ShelfCardProps {
  shelf: Shelf;
  isEditing?: boolean;
  onStartEdit?: () => void;
  onRename?: (id: string, name: string) => Promise<void>;
  onTogglePrivacy?: (shelf: Shelf) => Promise<void>;
  onDelete?: (id: string) => Promise<void>;
}

function ShelfCard({
  shelf,
  isEditing,
  onStartEdit,
  onRename,
  onTogglePrivacy,
  onDelete,
}: ShelfCardProps) {
  const [editName, setEditName] = useState(shelf.name);

  const handleRenameSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (onRename) {
      await onRename(shelf.id, editName);
    }
  };

  return (
    <div className="rounded-xl border bg-card p-6 text-card-foreground shadow-sm transition hover:-translate-y-0.5 hover:shadow-card">
      {isEditing ? (
        <form onSubmit={handleRenameSubmit} className="flex gap-2">
          <input
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
            className="flex-1 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
          <Button type="submit" size="sm">
            Сохранить
          </Button>
        </form>
      ) : (
        <Link
          href={`/shelves/${shelf.id}`}
          className="text-base font-semibold leading-tight hover:underline"
        >
          {shelf.name}
        </Link>
      )}

      {shelf.type === 'custom' && !shelf.isPublic && (
        <span className="mt-2 inline-block rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
          Приватная
        </span>
      )}

      {shelf.type === 'custom' && shelf.description && (
        <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{shelf.description}</p>
      )}

      <p className="mt-2 text-sm text-muted-foreground">
        {shelf.booksCount} {pluralizeBooks(shelf.booksCount)}
      </p>

      {shelf.type === 'custom' && !isEditing && (
        <div className="mt-4 flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={onStartEdit}>
            Переименовать
          </Button>
          <Button variant="outline" size="sm" onClick={() => onTogglePrivacy?.(shelf)}>
            {shelf.isPublic ? 'Сделать приватной' : 'Сделать публичной'}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onDelete?.(shelf.id)}
          >
            Удалить
          </Button>
        </div>
      )}
    </div>
  );
}

function pluralizeBooks(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'книга';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'книги';
  return 'книг';
}
