'use client';

import { listsApi } from '@/entities/list';
import { Book, List } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const editListSchema = z.object({
  title: z.string().trim().min(2, 'Минимум 2 символа').max(255, 'Максимум 255 символов'),
  description: z.string().trim().max(2000, 'Максимум 2000 символов'),
});

type EditListForm = z.infer<typeof editListSchema>;

interface ListDetailViewProps {
  id: string;
}

export function ListDetailView({ id }: ListDetailViewProps) {
  const router = useRouter();
  const [list, setList] = useState<List | null>(null);
  const currentIdRef = useRef(id);
  currentIdRef.current = id;
  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(12);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [shareSuccess, setShareSuccess] = useState(false);
  const shareTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<EditListForm>({
    resolver: zodResolver(editListSchema),
  });

  const fetchList = useCallback(
    async (targetPage: number, context: 'initial' | 'refresh') => {
      const requestedId = id;
      setLoading(true);
      if (context === 'initial') {
        setLoadError(null);
      } else {
        setMutationError(null);
      }
      try {
        const response = await listsApi.get(requestedId, { page: targetPage, limit });
        if (requestedId !== currentIdRef.current) {
          return;
        }
        setList(response.list);
        setBooks(response.items);
        setTotal(response.total);
        setPage(response.page);
        reset({
          title: response.list.title,
          description: response.list.description,
        });
      } catch (err) {
        if (requestedId !== currentIdRef.current) {
          return;
        }
        if (context === 'initial') {
          setLoadError(mapErrorToRussian(err));
        } else {
          setMutationError(mapErrorToRussian(err, 'Не удалось обновить данные списка'));
        }
      } finally {
        if (requestedId === currentIdRef.current) {
          setLoading(false);
        }
      }
    },
    [id, limit, reset]
  );

  useEffect(() => {
    setList(null);
    setBooks([]);
    setTotal(0);
    setPage(1);
    setLoadError(null);
    setMutationError(null);
    setIsEditing(false);
    setShareSuccess(false);
    if (shareTimeoutRef.current) {
      clearTimeout(shareTimeoutRef.current);
      shareTimeoutRef.current = null;
    }
    fetchList(1, 'initial');
  }, [id, fetchList]);

  useEffect(() => {
    return () => {
      if (shareTimeoutRef.current) {
        clearTimeout(shareTimeoutRef.current);
        shareTimeoutRef.current = null;
      }
    };
  }, []);

  const onUpdate = async (data: EditListForm) => {
    const listId = currentIdRef.current;
    setMutationError(null);
    try {
      await listsApi.update(listId, {
        title: data.title,
        description: data.description,
      });
      if (listId !== currentIdRef.current) {
        return;
      }
      setIsEditing(false);
      await fetchList(page, 'refresh');
    } catch (err) {
      if (listId !== currentIdRef.current) {
        return;
      }
      setMutationError(mapErrorToRussian(err, 'Ошибка обновления списка'));
    }
  };

  const onToggleVisibility = async () => {
    if (!list) return;
    const listId = currentIdRef.current;
    setMutationError(null);
    try {
      await listsApi.update(listId, {
        is_public: !list.isPublic,
      });
      if (listId !== currentIdRef.current) {
        return;
      }
      await fetchList(page, 'refresh');
    } catch (err) {
      if (listId !== currentIdRef.current) {
        return;
      }
      setMutationError(mapErrorToRussian(err, 'Ошибка изменения видимости списка'));
    }
  };

  const onDelete = async () => {
    if (!confirm('Удалить список? Книги будут убраны из него.')) {
      return;
    }
    const listId = currentIdRef.current;
    setMutationError(null);
    try {
      await listsApi.remove(listId);
      if (listId !== currentIdRef.current) {
        return;
      }
      router.push('/lists');
    } catch (err) {
      if (listId !== currentIdRef.current) {
        return;
      }
      setMutationError(mapErrorToRussian(err, 'Ошибка удаления списка'));
    }
  };

  const onRemoveBook = async (bookId: string) => {
    const listId = currentIdRef.current;
    setMutationError(null);
    try {
      await listsApi.removeBook(listId, bookId);
      if (listId !== currentIdRef.current) {
        return;
      }
      await fetchList(page, 'refresh');
    } catch (err) {
      if (listId !== currentIdRef.current) {
        return;
      }
      setMutationError(mapErrorToRussian(err, 'Ошибка удаления книги из списка'));
    }
  };

  const onShare = async () => {
    const listId = currentIdRef.current;
    setMutationError(null);
    const url = `${window.location.origin}/lists/public/${listId}`;
    try {
      await navigator.clipboard.writeText(url);
      if (listId !== currentIdRef.current) {
        return;
      }
      if (shareTimeoutRef.current) {
        clearTimeout(shareTimeoutRef.current);
      }
      setShareSuccess(true);
      shareTimeoutRef.current = setTimeout(() => {
        shareTimeoutRef.current = null;
        if (listId === currentIdRef.current) {
          setShareSuccess(false);
        }
      }, 2000);
    } catch {
      if (listId !== currentIdRef.current) {
        return;
      }
      setMutationError('Не удалось скопировать ссылку');
    }
  };

  if (loading && !list) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  if (loadError) {
    return (
      <main className="container mx-auto px-4 py-8">
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {loadError}
        </div>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-8">
      {isEditing && list ? (
        <form onSubmit={handleSubmit(onUpdate)} className="max-w-md space-y-3">
          <div>
            <input
              {...register('title')}
              className="w-full rounded-md border bg-background px-3 py-2 text-3xl font-bold outline-none focus:ring-2 focus:ring-primary"
            />
            {errors.title && (
              <p className="mt-1 text-sm text-destructive">{errors.title.message}</p>
            )}
          </div>
          <div>
            <textarea
              {...register('description')}
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
          <div>
            <h1 className="text-3xl font-bold tracking-tight">{list?.title}</h1>
            {list?.description && (
              <p className="mt-2 text-muted-foreground">{list.description}</p>
            )}
          </div>
          <div className="flex gap-2">
            {list?.isPublic && (
              <Button variant="outline" onClick={onShare}>
                {shareSuccess ? 'Скопировано!' : 'Поделиться'}
              </Button>
            )}
            <Button variant="outline" onClick={onToggleVisibility}>
              {list?.isPublic ? 'Сделать приватным' : 'Сделать публичным'}
            </Button>
            <Button variant="outline" onClick={() => setIsEditing(true)}>
              Редактировать
            </Button>
            <Button variant="destructive" onClick={onDelete}>
              Удалить
            </Button>
          </div>
        </div>
      )}

      {mutationError && (
        <div className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {mutationError}
        </div>
      )}

      <p className="mt-2 text-muted-foreground">
        {total} {pluralizeBooks(total)}
      </p>

      {books.length === 0 && !loading ? (
        <p className="mt-8 text-muted-foreground">В этом списке пока нет книг</p>
      ) : (
        <>
          <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {books.map((book) => (
              <div key={book.id} className="group relative">
                <Link href={`/books/${book.slug}`}>
                  <div className="relative aspect-[2/3] overflow-hidden rounded-lg bg-muted">
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
                  <h3 className="mt-2 line-clamp-2 text-sm font-medium">{book.title}</h3>
                  <p className="line-clamp-1 text-xs text-muted-foreground">
                    {book.authors.map((a) => a.name).join(', ')}
                  </p>
                  {book.genres.length > 0 && (
                    <p className="line-clamp-1 text-xs text-muted-foreground">
                      {book.genres.map((g) => g.name).join(', ')}
                    </p>
                  )}
                </Link>
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-2 w-full"
                  onClick={() => onRemoveBook(book.id)}
                >
                  Убрать из списка
                </Button>
              </div>
            ))}
          </div>

          <div className="mt-8 flex items-center justify-center gap-4">
            <Button
              variant="outline"
              onClick={() => fetchList(page - 1, 'refresh')}
              disabled={page <= 1 || loading}
            >
              Назад
            </Button>
            <span className="text-sm text-muted-foreground">
              Страница {page} из {Math.max(1, Math.ceil(total / limit))}
            </span>
            <Button
              variant="outline"
              onClick={() => fetchList(page + 1, 'refresh')}
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

function mapErrorToRussian(err: unknown, fallback = 'Ошибка загрузки списка'): string {
  if (err instanceof Error) {
    const message = err.message.toLowerCase();
    if (message.includes('invalid list id')) {
      return 'Некорректный идентификатор списка';
    }
    if (message.includes('list not found')) {
      return 'Список не найден или недоступен';
    }
    if (message.includes('forbidden')) {
      return 'Нет доступа к списку';
    }
  }
  return fallback;
}
