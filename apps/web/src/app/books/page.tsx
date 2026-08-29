import { api } from '@bookhaven/api-client';
import Image from 'next/image';
import Link from 'next/link';

export default async function BooksPage({
  searchParams,
}: {
  searchParams: { page?: string; search?: string };
}) {
  const page = Number(searchParams.page) || 1;
  const search = searchParams.search || '';

  const { items: books, total, limit } = await api.books.list({
    page,
    limit: 20,
    search,
  });

  const totalPages = Math.ceil(total / limit);

  return (
    <main className="container mx-auto px-4 py-8">
      <h1 className="mb-8 text-3xl font-bold tracking-tight sm:text-4xl">
        Каталог книг
      </h1>

      <div className="mb-8 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {books.map((book) => (
          <article
            key={book.id}
            className="group overflow-hidden rounded-lg border bg-card text-card-foreground shadow-sm transition-shadow hover:shadow-md"
          >
            <Link href={`/books/${book.slug}`} className="block">
              <div className="relative aspect-[2/3] overflow-hidden bg-muted">
                {book.coverUrl ? (
                  <Image
                    src={book.coverUrl}
                    alt={book.title}
                    fill
                    className="object-cover transition-transform group-hover:scale-105"
                    sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 25vw"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-muted-foreground">
                    Нет обложки
                  </div>
                )}
              </div>
              <div className="p-4">
                <h2 className="line-clamp-2 font-semibold leading-tight">
                  {book.title}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {book.authors.map((author) => author.name).join(', ')}
                </p>
                {book.rating > 0 && (
                  <p className="mt-2 text-sm font-medium">
                    Рейтинг: {book.rating.toFixed(1)}
                  </p>
                )}
              </div>
            </Link>
          </article>
        ))}
      </div>

      {totalPages > 1 && (
        <div className="flex justify-center gap-2">
          {page > 1 && (
            <Link
              href={`/books?page=${page - 1}${search ? `&search=${encodeURIComponent(search)}` : ''}`}
              className="rounded-md border px-4 py-2 hover:bg-accent"
            >
              Назад
            </Link>
          )}
          <span className="px-4 py-2">
            Страница {page} из {totalPages}
          </span>
          {page < totalPages && (
            <Link
              href={`/books?page=${page + 1}${search ? `&search=${encodeURIComponent(search)}` : ''}`}
              className="rounded-md border px-4 py-2 hover:bg-accent"
            >
              Вперед
            </Link>
          )}
        </div>
      )}
    </main>
  );
}
