'use client';

import { api, List } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const createListSchema = z.object({
  title: z.string().trim().min(2, 'Минимум 2 символа').max(255, 'Максимум 255 символов'),
  description: z.string().trim().max(2000, 'Максимум 2000 символов').optional(),
});

type CreateListForm = z.infer<typeof createListSchema>;

export default function ListsPage() {
  const [lists, setLists] = useState<List[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateListForm>({
    resolver: zodResolver(createListSchema),
  });

  const fetchLists = async () => {
    try {
      const response = await api.lists.list();
      setLists(response.items);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка загрузки списков');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLists();
  }, []);

  const onCreate = async (data: CreateListForm) => {
    setError(null);
    try {
      await api.lists.create({
        title: data.title,
        description: data.description,
        is_public: true,
      });
      reset();
      await fetchLists();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка создания списка');
    }
  };

  const onUpdate = async (id: string, title: string, description: string, isPublic: boolean) => {
    setError(null);
    try {
      await api.lists.update(id, { title, description, is_public: isPublic });
      setEditingId(null);
      await fetchLists();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка обновления списка');
    }
  };

  const onDelete = async (id: string) => {
    if (!confirm('Удалить список? Книги будут убраны из него.')) {
      return;
    }
    setError(null);
    try {
      await api.lists.remove(id);
      await fetchLists();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления списка');
    }
  };

  if (loading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold tracking-tight">Мои списки</h1>

      {error && (
        <div className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit(onCreate)} className="mt-6 max-w-md space-y-3">
        <div>
          <input
            {...register('title')}
            placeholder="Название нового списка"
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
          {errors.title && (
            <p className="mt-1 text-sm text-destructive">{errors.title.message}</p>
          )}
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
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Создание...' : 'Создать список'}
        </Button>
      </form>

      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {lists.map((list) => (
          <ListCard
            key={list.id}
            list={list}
            isEditing={editingId === list.id}
            onStartEdit={() => setEditingId(list.id)}
            onUpdate={onUpdate}
            onDelete={onDelete}
          />
        ))}
      </div>

      {lists.length === 0 && (
        <p className="mt-8 text-muted-foreground">
          У вас пока нет списков. Создайте первый!
        </p>
      )}
    </main>
  );
}

interface ListCardProps {
  list: List;
  isEditing?: boolean;
  onStartEdit?: () => void;
  onUpdate?: (id: string, title: string, description: string, isPublic: boolean) => Promise<void>;
  onDelete?: (id: string) => Promise<void>;
}

function ListCard({
  list,
  isEditing,
  onStartEdit,
  onUpdate,
  onDelete,
}: ListCardProps) {
  const [editTitle, setEditTitle] = useState(list.title);
  const [editDescription, setEditDescription] = useState(list.description);
  const [editIsPublic, setEditIsPublic] = useState(list.isPublic);

  const handleUpdateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (onUpdate) {
      await onUpdate(list.id, editTitle, editDescription, editIsPublic);
    }
  };

  return (
    <div className="rounded-lg border bg-card p-6 text-card-foreground shadow-sm">
      {isEditing ? (
        <form onSubmit={handleUpdateSubmit} className="space-y-3">
          <input
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
          <textarea
            value={editDescription}
            onChange={(e) => setEditDescription(e.target.value)}
            rows={2}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={editIsPublic}
              onChange={(e) => setEditIsPublic(e.target.checked)}
            />
            Публичный список
          </label>
          <div className="flex gap-2">
            <Button type="submit" size="sm">
              Сохранить
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onStartEdit && onStartEdit()}
            >
              Отмена
            </Button>
          </div>
        </form>
      ) : (
        <>
          <Link
            href={`/lists/${list.id}`}
            className="text-xl font-semibold hover:underline"
          >
            {list.title}
          </Link>
          {list.description && (
            <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">
              {list.description}
            </p>
          )}
          <p className="mt-2 text-sm text-muted-foreground">
            {list.booksCount} {pluralizeBooks(list.booksCount)}
          </p>
          <div className="mt-2 flex items-center gap-2">
            <span
              className={`inline-block rounded-full px-2 py-0.5 text-xs ${
                list.isPublic
                  ? 'bg-green-100 text-green-800'
                  : 'bg-gray-100 text-gray-800'
              }`}
            >
              {list.isPublic ? 'Публичный' : 'Приватный'}
            </span>
          </div>
          <div className="mt-4 flex gap-2">
            <Button variant="outline" size="sm" onClick={onStartEdit}>
              Редактировать
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => onDelete?.(list.id)}
            >
              Удалить
            </Button>
          </div>
        </>
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
