import { booksApi, BookCard } from '@/entities/book';
import { genresApi } from '@/entities/genre';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';

export async function HomeView() {
  const [{ items: books }, { items: genres }] = await Promise.all([
    booksApi.list({ limit: 10 }),
    genresApi.list(),
  ]);

  return (
    <main>
      <section className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <h1 className="font-display text-5xl font-semibold tracking-tight md:text-6xl">
            BookHaven
          </h1>
          <p className="mt-6 text-sm text-muted-foreground md:text-base">
            Социальный каталог книг с оценками, ревью и подборками
          </p>
          <div className="mt-8 flex justify-center gap-4">
            <Button size="lg" asChild>
              <Link href="/books">Найти книгу</Link>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <Link href="/shelves">Мои полки</Link>
            </Button>
          </div>
        </div>

        {genres.length > 0 && (
          <div className="mt-10 flex gap-2 overflow-x-auto pb-2">
            {genres.slice(0, 12).map((genre) => (
              <Link
                key={genre.id}
                href={`/books?genre_id=${genre.id}`}
                className="shrink-0 rounded-full border bg-card px-4 py-2 text-sm font-medium text-card-foreground shadow-sm transition hover:-translate-y-0.5 hover:shadow-card"
              >
                {genre.name}
              </Link>
            ))}
          </div>
        )}
      </section>

      <section className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
        <h2 className="font-display mb-8 text-2xl font-semibold tracking-tight md:text-3xl">
          Популярные книги
        </h2>
        <div className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          {books.map((book) => (
            <BookCard key={book.id} book={book} />
          ))}
        </div>
      </section>
    </main>
  );
}
