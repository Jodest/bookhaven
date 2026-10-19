'use client';

import { useAuthStore } from '@/entities/session';
import { shelfKeys, shelvesApi } from '@/entities/shelf';
import { invalidateShelfContents, invalidateShelfMeta } from '@/features/shelve-book';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Book, Shelf } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const editShelfSchema = z.object({
  name: z.string().trim().min(2, 'Минимум 2 символа').max(100, 'Максимум 100 символов'),
  description: z.string().trim().max(2000, 'Максимум 2000 символов'),
});

type EditShelfForm = z.infer<typeof editShelfSchema>;

interface ShelfDetailViewProps {
  id: string;
}

const LIMIT = 12;

export function ShelfDetailView({ id }: ShelfDetailViewProps) {
  const router = useRouter();
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const limit = LIMIT;
  const [actionError, setActionError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<EditShelfForm>({
    resolver: zodResolver(editShelfSchema),
  });

  const query = useQuery({
    queryKey: shelfKeys.detailPage(user?.id, id, page, limit),
    queryFn: () => shelvesApi.get(id, { page, limit }),
    placeholderData: keepPreviousData,
  });
  const data = query.data;
  const shelf: Shelf | null = data?.shelf ?? null;
  const books: Book[] = data?.items ?? [];
  const total = data?.total ?? 0;
  const loading = query.isFetching;
  const error =
    actionError ??
    (query.isError
      ? query.error instanceof Error
        ? query.error.message
        : 'Ошибка загрузки полки'
      : null);

  useEffect(() => {
    if (data?.shelf && !isEditing) {
      reset({ name: data.shelf.name, description: data.shelf.description });
    }
  }, [data?.shelf, isEditing, reset]);

  const updateMutation = useMutation({
    mutationFn: (body: { name?: string; description?: string; is_public?: boolean }) =>
      shelvesApi.update(id, body),
    onSuccess: () => invalidateShelfMeta(queryClient),
  });
  const removeShelfMutation = useMutation({
    mutationFn: () => shelvesApi.remove(id),
  });
  const removeBookMutation = useMutation({
    mutationFn: (bookId: string) => shelvesApi.removeBook(id, bookId),
    onSuccess: () => invalidateShelfContents(queryClient),
  });

  const messageOf = (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback;

  const onEdit = async (formData: EditShelfForm) => {
    setActionError(null);
    try {
      await updateMutation.mutateAsync({ name: formData.name, description: formData.description });
      setIsEditing(false);
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка сохранения полки'));
    }
  };

  const onTogglePrivacy = async () => {
    if (!shelf) return;
    setActionError(null);
    try {
      await updateMutation.mutateAsync({ is_public: !shelf.isPublic });
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка изменения видимости полки'));
    }
  };

  const onDelete = async () => {
    if (!confirm('Удалить полку? Книги будут убраны из неё.')) {
      return;
    }
    setActionError(null);
    try {
      await removeShelfMutation.mutateAsync();
      // Drop the deleted shelf first so invalidation does not refetch it (404).
      queryClient.removeQueries({ queryKey: shelfKeys.detail(user?.id, id) });
      await invalidateShelfContents(queryClient);
      router.push('/shelves');
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка удаления полки'));
    }
  };

  const onRemoveBook = async (bookId: string) => {
    setActionError(null);
    try {
      await removeBookMutation.mutateAsync(bookId);
    } catch (err) {
      setActionError(messageOf(err, 'Ошибка удаления книги'));
    }
  };

  if (query.isPending) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      {isEditing && shelf ? (
        <form onSubmit={handleSubmit(onEdit)} className="max-w-md space-y-2">
          <div>
            <input
              {...register('name')}
              aria-label="Название"
              className="w-full rounded-md border bg-background px-3 py-2 text-3xl font-bold outline-none focus:ring-2 focus:ring-primary"
            />
            {errors.name && (
              <p className="mt-1 text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <div>
            <textarea
              {...register('description')}
              aria-label="Описание"
              placeholder="Описание (необязательно)"
              rows={3}
              className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
            />
            {errors.description && (
              <p className="mt-1 text-sm text-destructive">{errors.description.message}</p>
            )}
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Сохранение...' : 'Сохранить'}
            </Button>
            <Button type="button" variant="outline" onClick={() => setIsEditing(false)}>
              Отмена
            </Button>
          </div>
        </form>
      ) : (
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">
            {shelf?.name}
            {shelf?.type === 'custom' && !shelf.isPublic && (
              <span className="ml-3 inline-block rounded-full bg-muted px-2 py-0.5 align-middle text-xs font-normal text-muted-foreground">
                Приватная
              </span>
            )}
          </h1>
          {shelf?.type === 'custom' && (
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => setIsEditing(true)}>
                Редактировать
              </Button>
              <Button variant="outline" onClick={onTogglePrivacy}>
                {shelf.isPublic ? 'Сделать приватной' : 'Сделать публичной'}
              </Button>
              <Button variant="destructive" onClick={onDelete}>
                Удалить
              </Button>
            </div>
          )}
        </div>
      )}

      {error && (
        <div className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {shelf?.type === 'custom' && shelf.description && !isEditing && (
        <p className="mt-2 max-w-2xl whitespace-pre-line text-muted-foreground">
          {shelf.description}
        </p>
      )}

      <p className="mt-2 text-muted-foreground">
        {total} {pluralizeBooks(total)}
      </p>

      {books.length === 0 && !loading ? (
        <p className="mt-8 text-muted-foreground">На этой полке пока нет книг</p>
      ) : (
        <>
          <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {books.map((book) => (
              <div key={book.id} className="group relative">
                <Link href={`/books/${book.slug}`}>
                  <div className="relative aspect-[2/3] overflow-hidden rounded-xl bg-muted transition group-hover:-translate-y-0.5 group-hover:shadow-card">
                    {book.coverUrl ? (
                      <Image
                        src={book.coverUrl}
                        alt={book.title}
                        fill
                        className="object-cover"
                        sizes="(max-width: 768px) 50vw, 200px"
                      />
                    ) : (
                      <div className="flex h-full items-center justify-center text-muted-foreground">
                        Нет обложки
                      </div>
                    )}
                  </div>
                  <h3 className="mt-2 line-clamp-2 text-sm font-semibold leading-tight">
                    {book.title}
                  </h3>
                  <p className="line-clamp-1 text-xs text-muted-foreground">
                    {book.authors.map((a) => a.name).join(', ')}
                  </p>
                </Link>
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-2 w-full"
                  onClick={() => onRemoveBook(book.id)}
                >
                  Убрать с полки
                </Button>
              </div>
            ))}
          </div>

          <div className="mt-8 flex items-center justify-center gap-4">
            <Button
              variant="outline"
              onClick={() => setPage((p) => p - 1)}
              disabled={page <= 1 || loading}
            >
              Назад
            </Button>
            <span className="text-sm text-muted-foreground">
              Страница {page} из {Math.max(1, Math.ceil(total / limit))}
            </span>
            <Button
              variant="outline"
              onClick={() => setPage((p) => p + 1)}
              disabled={page * limit >= total || loading}
            >
              Вперёд
            </Button>
          </div>
        </>
      )}
    </main>
  );
}

function pluralizeBooks(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'книга';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'книги';
  return 'книг';
}
