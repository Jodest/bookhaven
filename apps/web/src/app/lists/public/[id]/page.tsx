'use client';

import { api, Book, List } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';

interface PublicListPageProps {
  params: { id: string };
}

export default function PublicListPage({ params }: PublicListPageProps) {
  const [list, setList] = useState<List | null>(null);
  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(12);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchList = useCallback(
    async (targetPage: number) => {
      setLoading(true);
      setError(null);
      try {
        const response = await api.lists.getPublic(params.id, { page: targetPage, limit });
        setList(response.list);
        setBooks(response.items);
        setTotal(response.total);
        setPage(response.page);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Ошибка загрузки списка');
      } finally {
        setLoading(false);
      }
    },
    [params.id, limit]
  );

  useEffect(() => {
    fetchList(1);
  }, [fetchList]);

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
    <main className="container mx-auto px-4 py-8">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">{list?.title}</h1>
          {list?.description && (
            <p className="mt-2 text-muted-foreground">{list.description}</p>
          )}
        </div>
        <span className="rounded-full bg-green-100 px-3 py-1 text-sm text-green-800">
          Публичный список
        </span>
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
