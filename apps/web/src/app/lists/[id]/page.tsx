'use client';

import { api, Book, List } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const editListSchema = z.object({
  title: z.string().trim().min(2, 'Минимум 2 символа').max(255, 'Максимум 255 символов'),
  description: z.string().trim().max(2000, 'Максимум 2000 символов'),
});

type EditListForm = z.infer<typeof editListSchema>;

interface ListPageProps {
  params: { id: string };
}

export default function ListPage({ params }: ListPageProps) {
  const router = useRouter();
  const [list, setList] = useState<List | null>(null);
  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(12);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [shareSuccess, setShareSuccess] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<EditListForm>({
    resolver: zodResolver(editListSchema),
  });

  const fetchList = useCallback(
    async (targetPage: number) => {
      setLoading(true);
      setError(null);
      try {
        const response = await api.lists.get(params.id, { page: targetPage, limit });
        setList(response.list);
        setBooks(response.items);
        setTotal(response.total);
        setPage(response.page);
        reset({
          title: response.list.title,
          description: response.list.description,
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Ошибка загрузки списка');
      } finally {
        setLoading(false);
      }
    },
    [params.id, limit, reset]
  );

  useEffect(() => {
    fetchList(1);
  }, [fetchList]);

  const onUpdate = async (data: EditListForm) => {
    setError(null);
    try {
      await api.lists.update(params.id, {
        title: data.title,
        description: data.description,
      });
      setIsEditing(false);
      await fetchList(page);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка обновления списка');
    }
  };

  const onToggleVisibility = async () => {
    if (!list) return;
    setError(null);
    try {
      await api.lists.update(params.id, {
        is_public: !list.isPublic,
      });
      await fetchList(page);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка изменения видимости');
    }
  };

  const onDelete = async () => {
    if (!confirm('Удалить список? Книги будут убраны из него.')) {
      return;
    }
    setError(null);
    try {
      await api.lists.remove(params.id);
      router.push('/lists');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления списка');
    }
  };

  const onRemoveBook = async (bookId: string) => {
    setError(null);
    try {
      await api.lists.removeBook(params.id, bookId);
      await fetchList(page);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления книги');
    }
  };

  const onShare = async () => {
    const url = `${window.location.origin}/lists/public/${params.id}`;
    try {
      await navigator.clipboard.writeText(url);
      setShareSuccess(true);
      setTimeout(() => setShareSuccess(false), 2000);
    } catch {
      setError('Не удалось скопировать ссылку');
    }
  };

  if (loading && !list) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
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

      {error && (
        <div className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
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
              onClick={() => fetchList(page - 1)}
              disabled={page <= 1 || loading}
            >
              Назад
            </Button>
            <span className="text-sm text-muted-foreground">
              Страница {page} из {Math.max(1, Math.ceil(total / limit))}
            </span>
            <Button
              variant="outline"
              onClick={() => fetchList(page + 1)}
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
