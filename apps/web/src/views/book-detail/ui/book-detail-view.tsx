import { AuthorNameLink } from '@/entities/author';
import { booksApi } from '@/entities/book';
import { CollectionPlaceBadges, collectionsApi } from '@/entities/curated-collection';
import { BookAdaptations, screenWorksApi } from '@/entities/screen-work';
import { AddToListButton } from '@/features/add-to-list';
import { BookRating } from '@/features/rate-book';
import { ShelveBookButtons } from '@/features/shelve-book';
import { ReviewsSection } from '@/widgets/reviews-section';
import { SimilarBooks } from '@/widgets/similar-books';
import { Badge } from '@bookhaven/ui';
import { Star } from 'lucide-react';
import Image from 'next/image';
import { notFound } from 'next/navigation';

interface BookDetailViewProps {
  slug: string;
}

export async function BookDetailView({ slug }: BookDetailViewProps) {
  const book = await booksApi.get(slug).catch(() => null);

  if (!book) {
    notFound();
  }

  // Places in curated collections (openspec collections/curated-collections);
  // a failure only hides the badges.
  const places = await collectionsApi.forBook(book.id).catch(() => null);

  // Screen adaptations (openspec screen-works/book-adaptations); a failure
  // only hides the block.
  const adaptations = await screenWorksApi.forBook(book.id).catch(() => null);

  // The book's own description always wins; an approved AI description
  // (openspec catalog-admin/ai-descriptions) is only a labelled fallback.
  const ownDescription = book.description?.trim() ? book.description : null;
  const aiDescriptionText = ownDescription ? null : book.aiDescription?.text?.trim() || null;
  const descriptionText = ownDescription ?? aiDescriptionText;
  const isAIDescription = !ownDescription && !!aiDescriptionText;

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <div className="grid grid-cols-1 gap-8 md:grid-cols-[300px_1fr]">
        <div className="relative aspect-[2/3] overflow-hidden rounded-xl bg-muted">
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
          {book.rating > 0 && (
            <div className="absolute left-2 top-2 inline-flex items-center gap-1 rounded-full bg-highlight px-2 py-0.5 text-xs font-semibold text-highlight-foreground shadow-sm">
              <Star className="h-3 w-3 fill-current" />
              {book.rating.toFixed(1)}
            </div>
          )}
        </div>

        <div>
          <h1 className="font-display text-3xl font-semibold tracking-tight sm:text-4xl">
            {book.title}
          </h1>

          <div className="mt-4 flex flex-wrap gap-2">
            {book.authors.map((author) => (
              <AuthorNameLink
                key={author.id}
                author={author}
                className="text-lg"
                linkClassName="text-primary hover:underline"
              />
            ))}
          </div>

          <div className="mt-4">
            <CollectionPlaceBadges places={places?.items ?? []} />
          </div>

          <div className="mt-6">
            <ShelveBookButtons bookId={book.id} slug={book.slug} />
          </div>

          <div className="mt-6">
            <AddToListButton bookId={book.id} slug={book.slug} />
          </div>

          <div className="mt-6">
            <BookRating
              bookId={book.id}
              slug={book.slug}
              rating={book.rating}
              ratingsCount={book.ratingsCount}
            />
          </div>

          {descriptionText && (
            <div className="mt-6">
              <h2 className="font-display text-xl font-semibold">Описание</h2>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground md:text-base">
                {descriptionText}
              </p>
              {isAIDescription && (
                <p className="mt-2 text-xs text-muted-foreground">Описание создано ИИ и может содержать неточности.</p>
              )}
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
              <Badge key={genre.id} variant="outline">
                {genre.name}
              </Badge>
            ))}
          </div>

          <BookAdaptations items={adaptations?.items ?? []} />

          <ReviewsSection bookId={book.id} slug={book.slug} />
        </div>
      </div>

      <SimilarBooks bookId={book.id} className="mt-12" />
    </main>
  );
}
