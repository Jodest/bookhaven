'use client';

import { shelvesApi } from '@/entities/shelf';
import { Book, Shelf } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const renameShelfSchema = z.object({
  name: z.string().trim().min(2, 'Минимум 2 символа').max(100, 'Максимум 100 символов'),
});

type RenameShelfForm = z.infer<typeof renameShelfSchema>;

interface ShelfDetailViewProps {
  id: string;
}

export function ShelfDetailView({ id }: ShelfDetailViewProps) {
  const router = useRouter();
  const [shelf, setShelf] = useState<Shelf | null>(null);
  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(12);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<RenameShelfForm>({
    resolver: zodResolver(renameShelfSchema),
  });

  const fetchShelf = useCallback(
    async (targetPage: number) => {
      setLoading(true);
      setError(null);
      try {
        const response = await shelvesApi.get(id, { page: targetPage, limit });
        setShelf(response.shelf);
        setBooks(response.items);
        setTotal(response.total);
        setPage(response.page);
        reset({ name: response.shelf.name });
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Ошибка загрузки полки');
      } finally {
        setLoading(false);
      }
    },
    [id, limit, reset]
  );

  useEffect(() => {
    fetchShelf(1);
  }, [fetchShelf]);

  const onRename = async (data: RenameShelfForm) => {
    setError(null);
    try {
      await shelvesApi.rename(id, { name: data.name });
      setIsEditing(false);
      await fetchShelf(page);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка переименования полки');
    }
  };

  const onDelete = async () => {
    if (!confirm('Удалить полку? Книги будут убраны из неё.')) {
      return;
    }
    setError(null);
    try {
      await shelvesApi.remove(id);
      router.push('/shelves');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления полки');
    }
  };

  const onRemoveBook = async (bookId: string) => {
    setError(null);
    try {
      await shelvesApi.removeBook(id, bookId);
      await fetchShelf(page);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка удаления книги');
    }
  };

  if (loading && !shelf) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-8">
      {isEditing && shelf ? (
        <form onSubmit={handleSubmit(onRename)} className="flex max-w-md items-start gap-2">
          <div className="flex-1">
            <input
              {...register('name')}
              className="w-full rounded-md border bg-background px-3 py-2 text-3xl font-bold outline-none focus:ring-2 focus:ring-primary"
            />
            {errors.name && (
              <p className="mt-1 text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Сохранение...' : 'Сохранить'}
          </Button>
          <Button type="button" variant="outline" onClick={() => setIsEditing(false)}>
            Отмена
          </Button>
        </form>
      ) : (
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h1 className="text-3xl font-bold tracking-tight">{shelf?.name}</h1>
          {shelf?.type === 'custom' && (
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setIsEditing(true)}>
                Переименовать
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
                  Убрать с полки
                </Button>
              </div>
            ))}
          </div>

          <div className="mt-8 flex items-center justify-center gap-4">
            <Button
              variant="outline"
              onClick={() => fetchShelf(page - 1)}
              disabled={page <= 1 || loading}
            >
              Назад
            </Button>
            <span className="text-sm text-muted-foreground">
              Страница {page} из {Math.max(1, Math.ceil(total / limit))}
            </span>
            <Button
              variant="outline"
              onClick={() => fetchShelf(page + 1)}
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
