'use client';

import { listsApi } from '@/entities/list';
import { Book, PublicList } from '@bookhaven/api-client';
import { Badge, Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';

interface PublicListDetailViewProps {
  id: string;
}

export function PublicListDetailView({ id }: PublicListDetailViewProps) {
  const [list, setList] = useState<PublicList | null>(null);
  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(12);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const currentIdRef = useRef(id);
  currentIdRef.current = id;

  const fetchList = useCallback(
    async (targetPage: number) => {
      const requestedId = id;
      setLoading(true);
      setError(null);
      try {
        const response = await listsApi.getPublic(requestedId, { page: targetPage, limit });
        if (requestedId !== currentIdRef.current) {
          return;
        }
        setList(response.list);
        setBooks(response.items);
        setTotal(response.total);
        setPage(response.page);
      } catch (err) {
        if (requestedId !== currentIdRef.current) {
          return;
        }
        setError(mapErrorToRussian(err));
      } finally {
        if (requestedId === currentIdRef.current) {
          setLoading(false);
        }
      }
    },
    [id, limit]
  );

  useEffect(() => {
    setList(null);
    setBooks([]);
    setTotal(0);
    setPage(1);
    setError(null);
    fetchList(1);
  }, [id, fetchList]);

  if (loading && !list) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  if (error) {
    return (
      <main className="container mx-auto px-4 py-8">
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">
            {list?.title}
          </h1>
          {list?.description && (
            <p className="mt-2 text-muted-foreground">{list.description}</p>
          )}
        </div>
        <Badge variant="outline">Публичный список</Badge>
      </div>

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
                  {book.genres.length > 0 && (
                    <p className="line-clamp-1 text-xs text-muted-foreground">
                      {book.genres.map((g) => g.name).join(', ')}
                    </p>
                  )}
                </Link>
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

function mapErrorToRussian(err: unknown): string {
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
  return 'Ошибка загрузки списка';
}
