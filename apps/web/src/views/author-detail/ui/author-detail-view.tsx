import { authorsApi, isWikimediaUrl, PhotoAttribution } from '@/entities/author';
import { BookCard } from '@/entities/book';
import { AuthorFilms, screenWorksApi } from '@/entities/screen-work';
import Image from 'next/image';
import { notFound } from 'next/navigation';

interface AuthorDetailViewProps {
  slug: string;
}

export async function AuthorDetailView({ slug }: AuthorDetailViewProps) {
  const author = await authorsApi.get(slug).catch(() => null);

  if (!author) {
    notFound();
  }

  // Films about the author (openspec screen-works/author-films); a failure
  // only hides the block.
  const films = await screenWorksApi.forAuthor(author.id).catch(() => null);

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <div className="grid grid-cols-1 gap-8 md:grid-cols-[200px_1fr]">
        <div>
          <div className="relative aspect-square overflow-hidden rounded-xl bg-muted">
            {author.photoUrl ? (
              <Image
                src={author.photoUrl}
                alt={author.name}
                fill
                className="object-cover"
                sizes="(max-width: 768px) 100vw, 200px"
                priority
                unoptimized={isWikimediaUrl(author.photoUrl)}
              />
            ) : (
              <div className="flex h-full items-center justify-center text-muted-foreground">
                Нет фото
              </div>
            )}
          </div>
          {author.photoUrl && author.photoAttribution && (
            <PhotoAttribution attribution={author.photoAttribution} className="mt-2" />
          )}
        </div>

        <div>
          <h1 className="font-display text-3xl font-semibold tracking-tight sm:text-4xl">
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
              <h2 className="font-display text-xl font-semibold">Биография</h2>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground md:text-base">
                {author.biography}
              </p>
            </div>
          )}
        </div>
      </div>

      {author.books && author.books.length > 0 && (
        <section className="py-12 md:py-16">
          <h2 className="font-display mb-8 text-2xl font-semibold tracking-tight md:text-3xl">
            Книги автора
          </h2>
          <div className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
            {author.books.map((book) => (
              <BookCard key={book.id} book={book} />
            ))}
          </div>
        </section>
      )}

      <AuthorFilms items={films?.items ?? []} />
    </main>
  );
}
