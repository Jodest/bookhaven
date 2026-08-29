import { api } from '@bookhaven/api-client';
import { ShelveBookButtons } from '@/components/shelve-book-buttons';
import Image from 'next/image';
import Link from 'next/link';
import { notFound } from 'next/navigation';

interface BookPageProps {
  params: { slug: string };
}

export default async function BookPage({ params }: BookPageProps) {
  const book = await api.books.get(params.slug).catch(() => null);

  if (!book) {
    notFound();
  }

  return (
    <main className="container mx-auto px-4 py-8">
      <div className="grid grid-cols-1 gap-8 md:grid-cols-[300px_1fr]">
        <div className="relative aspect-[2/3] overflow-hidden rounded-lg bg-muted">
          {book.coverUrl ? (
            <Image
              src={book.coverUrl}
              alt={book.title}
              fill
              className="object-cover"
              sizes="(max-width: 768px) 100vw, 300px"
              priority
            />
          ) : (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              Нет обложки
            </div>
          )}
        </div>

        <div>
          <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">
            {book.title}
          </h1>

          <div className="mt-4 flex flex-wrap gap-2">
            {book.authors.map((author) => (
              <Link
                key={author.id}
                href={`/authors/${author.slug}`}
                className="text-lg text-primary hover:underline"
              >
                {author.name}
              </Link>
            ))}
          </div>

          <div className="mt-6">
            <ShelveBookButtons bookId={book.id} slug={book.slug} />
          </div>

          {book.rating > 0 && (
            <p className="mt-4 text-lg">
              Рейтинг: <span className="font-semibold">{book.rating.toFixed(1)}</span>
              <span className="text-muted-foreground"> ({book.ratingsCount} оценок)</span>
            </p>
          )}

          {book.description && (
            <div className="mt-6">
              <h2 className="text-xl font-semibold">Описание</h2>
              <p className="mt-2 leading-relaxed text-muted-foreground">
                {book.description}
              </p>
            </div>
          )}

          <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3">
            {book.publishedYear && (
              <div>
                <dt className="text-sm text-muted-foreground">Год издания</dt>
                <dd className="font-medium">{book.publishedYear}</dd>
              </div>
            )}
            {book.pageCount && (
              <div>
                <dt className="text-sm text-muted-foreground">Страниц</dt>
                <dd className="font-medium">{book.pageCount}</dd>
              </div>
            )}
            {book.isbn && (
              <div>
                <dt className="text-sm text-muted-foreground">ISBN</dt>
                <dd className="font-medium">{book.isbn}</dd>
              </div>
            )}
          </div>

          <div className="mt-6 flex flex-wrap gap-2">
            {book.genres.map((genre) => (
              <span
                key={genre.id}
                className="rounded-full border px-3 py-1 text-sm"
              >
                {genre.name}
              </span>
            ))}
          </div>
        </div>
      </div>
    </main>
  );
}
