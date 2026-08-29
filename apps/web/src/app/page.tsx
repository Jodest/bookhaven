import { api } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';

export default async function HomePage() {
  const { items: books } = await api.books.list({ limit: 12 });

  return (
    <main className="container mx-auto px-4 py-16">
      <div className="mx-auto max-w-2xl text-center">
        <h1 className="mb-6 text-4xl font-bold tracking-tight sm:text-6xl">
          BookHaven
        </h1>
        <p className="mb-8 text-lg text-muted-foreground">
          Социальный каталог книг с оценками, ревью и подборками
        </p>
        <div className="flex justify-center gap-4">
          <Button size="lg" asChild>
            <Link href="/books">Найти книгу</Link>
          </Button>
          <Button size="lg" variant="outline">
            Мои полки
          </Button>
        </div>
      </div>

      <section className="mt-16">
        <h2 className="mb-8 text-2xl font-semibold tracking-tight sm:text-3xl">
          Популярные книги
        </h2>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
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
                  <h3 className="line-clamp-2 font-semibold leading-tight">
                    {book.title}
                  </h3>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {book.authors.map((a) => a.name).join(', ')}
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
      </section>
    </main>
  );
}
