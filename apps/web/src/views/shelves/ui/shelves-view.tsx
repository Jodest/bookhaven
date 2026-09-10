'use client';

import { shelvesApi } from '@/entities/shelf';
import { Shelf } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const createShelfSchema = z.object({
  name: z.string().trim().min(2, 'Минимум 2 символа').max(100, 'Максимум 100 символов'),
});

type CreateShelfForm = z.infer<typeof createShelfSchema>;

const defaultTypeOrder = ['want_to_read', 'reading', 'read'];

export function ShelvesView() {
  const [shelves, setShelves] = useState<Shelf[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateShelfForm>({
    resolver: zodResolver(createShelfSchema),
  });

  const fetchShelves = async () => {
    try {
      const response = await shelvesApi.list();
      setShelves(response.items);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка загрузки полок');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchShelves();
  }, []);

  const onCreate = async (data: CreateShelfForm) => {
    setError(null);
    try {
      await shelvesApi.create({ name: data.name });
      reset();
      await fetchShelves();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка создания полки');
    }
  };

  const onRename = async (id: string, name: string) => {
    setError(null);
    try {
      await shelvesApi.rename(id, { name });
      setEditingId(null);
      await fetchShelves();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка переименования полки');
    }
  };

  const onDelete = async (id: string) => {
    if (!confirm('Удалить полку? Книги будут убраны из неё.')) {
      return;
    }
    setError(null);
    try {
      await shelvesApi.remove(id);
      await fetchShelves();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления полки');
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

      <form onSubmit={handleSubmit(onCreate)} className="mt-6 flex max-w-md gap-2">
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
  onDelete?: (id: string) => Promise<void>;
}

function ShelfCard({
  shelf,
  isEditing,
  onStartEdit,
  onRename,
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

      <p className="mt-2 text-sm text-muted-foreground">
        {shelf.booksCount} {pluralizeBooks(shelf.booksCount)}
      </p>

      {shelf.type === 'custom' && !isEditing && (
        <div className="mt-4 flex gap-2">
          <Button variant="outline" size="sm" onClick={onStartEdit}>
            Переименовать
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
