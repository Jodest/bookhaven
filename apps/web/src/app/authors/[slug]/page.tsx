import { api } from '@bookhaven/api-client';
import Image from 'next/image';
import Link from 'next/link';
import { notFound } from 'next/navigation';

interface AuthorPageProps {
  params: { slug: string };
}

export default async function AuthorPage({ params }: AuthorPageProps) {
  const author = await api.authors.get(params.slug).catch(() => null);

  if (!author) {
    notFound();
  }

  return (
    <main className="container mx-auto px-4 py-8">
      <div className="grid grid-cols-1 gap-8 md:grid-cols-[200px_1fr]">
        <div className="relative aspect-square overflow-hidden rounded-lg bg-muted">
          {author.photoUrl ? (
            <Image
              src={author.photoUrl}
              alt={author.name}
              fill
              className="object-cover"
              sizes="(max-width: 768px) 100vw, 200px"
              priority
            />
          ) : (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              Нет фото
            </div>
          )}
        </div>

        <div>
          <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">
            {author.name}
          </h1>

          {author.birthDate && (
            <p className="mt-2 text-muted-foreground">
              {author.birthDate}
              {author.deathDate && ` — ${author.deathDate}`}
            </p>
          )}

          {author.biography && (
            <div className="mt-6">
              <h2 className="text-xl font-semibold">Биография</h2>
              <p className="mt-2 leading-relaxed text-muted-foreground">
                {author.biography}
              </p>
            </div>
          )}
        </div>
      </div>

      {author.books && author.books.length > 0 && (
        <section className="mt-12">
          <h2 className="mb-6 text-2xl font-semibold">Книги автора</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {author.books.map((book) => (
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
      )}
    </main>
  );
}
