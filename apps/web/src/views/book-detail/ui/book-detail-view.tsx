import { AuthorNameLink } from '@/entities/author';
import { CollectionPlaceBadges, collectionsApi } from '@/entities/curated-collection';
import { BookAdaptations, screenWorksApi } from '@/entities/screen-work';
import { BookRating } from '@/features/rate-book';
import { BookShelfActions } from '@/features/shelve-book';
import { AdaptationVerdict, AdaptationVotePrompt } from '@/features/vote-adaptation';
import { languageNameRu } from '@/shared/lib/language-name';
import { pluralRu } from '@/shared/lib/plural-ru';
import { ReviewsSection } from '@/widgets/reviews-section';
import { SimilarBooks } from '@/widgets/similar-books';
import { Badge } from '@bookhaven/ui';
import { Clapperboard } from 'lucide-react';
import Image from 'next/image';
import Link from 'next/link';
import { notFound } from 'next/navigation';
import { getBook } from '../api/get-book';
import { buildBookJsonLd } from '../lib/book-seo';
import { BookDescription } from './book-description';
import { RatingSummary } from './rating-summary';

interface BookDetailViewProps {
  slug: string;
}

export async function BookDetailView({ slug }: BookDetailViewProps) {
  const book = await getBook(slug);

  if (!book) {
    notFound();
  }

  // Places in curated collections (openspec collections/curated-collections)
  // and screen adaptations (openspec screen-works/book-adaptations) are
  // fetched in parallel; a failure only hides the badges / the block.
  const [places, adaptations] = await Promise.all([
    collectionsApi.forBook(book.id).catch(() => null),
    screenWorksApi.forBook(book.id).catch(() => null),
  ]);

  // The book's own description always wins; an approved AI description
  // (openspec catalog-admin/ai-descriptions) is only a labelled fallback.
  const ownDescription = book.description?.trim() ? book.description : null;
  const aiDescriptionText = ownDescription ? null : book.aiDescription?.text?.trim() || null;
  const descriptionText = ownDescription ?? aiDescriptionText;
  const isAIDescription = !ownDescription && !!aiDescriptionText;

  const language = languageNameRu(book.language);
  type Detail = { term: string; value: string | number };
  const details = [
    book.publishedYear ? ({ term: 'Год издания', value: book.publishedYear } as Detail) : null,
    book.pageCount ? { term: 'Страниц', value: book.pageCount } : null,
    language ? { term: 'Язык', value: language } : null,
    book.isbn ? { term: 'ISBN', value: book.isbn } : null,
  ].filter((d): d is Detail => d !== null);

  const adaptationItems = adaptations?.items ?? [];
  const hasAdaptations = adaptationItems.length > 0;

  // `<` is escaped so a title/description can never close the script tag.
  const jsonLd = JSON.stringify(buildBookJsonLd(book))
    .replace(/</g, '\\u003c')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029');

  return (
    <main className="container mx-auto px-4 py-8 sm:px-6 md:py-16 lg:px-8">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd }} />

      <div className="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-4 gap-y-6 md:grid-cols-[260px_minmax(0,1fr)] md:grid-rows-[auto_1fr] md:gap-x-10">
        <aside className="max-md:contents md:col-start-1 md:row-span-2 md:row-start-1 md:flex md:flex-col md:gap-6 md:sticky md:top-20 md:max-h-[calc(100vh-6rem)] md:self-start md:overflow-y-auto">
          <div className="relative aspect-[2/3] overflow-hidden rounded-xl bg-muted max-md:col-start-1 max-md:row-start-1">
            {book.coverUrl ? (
              <Image
                src={book.coverUrl}
                alt={book.title}
                fill
                className="object-cover"
                sizes="(max-width: 767px) 40vw, 260px"
                priority
              />
            ) : (
              <div className="flex h-full items-center justify-center text-muted-foreground">
                Нет обложки
              </div>
            )}
          </div>

          <div className="space-y-4 max-md:col-span-2 max-md:row-start-2">
            <BookShelfActions bookId={book.id} slug={book.slug} />
            <BookRating bookId={book.id} slug={book.slug} />
          </div>
        </aside>

        <div className="space-y-3 max-md:col-start-2 max-md:row-start-1 md:col-start-2 md:row-start-1">
          <h1 className="font-display text-2xl font-semibold tracking-tight sm:text-3xl md:text-4xl">
            {book.title}
          </h1>

          <div className="flex flex-wrap gap-x-3 gap-y-1">
            {book.authors.map((author) => (
              <AuthorNameLink
                key={author.id}
                author={author}
                className="text-base md:text-lg"
                linkClassName="text-primary hover:underline"
              />
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <RatingSummary
              rating={book.rating}
              ratingsCount={book.ratingsCount}
              reviewsCount={book.reviewsCount}
            />
            {hasAdaptations && (
              <a
                href="#adaptations"
                className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
              >
                <Clapperboard className="h-4 w-4" aria-hidden="true" />
                {adaptationItems.length}{' '}
                {pluralRu(adaptationItems.length, ['экранизация', 'экранизации', 'экранизаций'])}
              </a>
            )}
          </div>
        </div>

        <div className="min-w-0 space-y-6 max-md:col-span-2 md:col-start-2 md:row-start-2">
          <CollectionPlaceBadges places={places?.items ?? []} />

          {descriptionText && (
            <div>
              <h2 className="font-display text-xl font-semibold">Описание</h2>
              <div className="mt-2">
                <BookDescription text={descriptionText} isAI={isAIDescription} />
              </div>
            </div>
          )}

          {details.length > 0 && (
            <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4">
              {details.map((d) => (
                <div key={d.term}>
                  <dt className="text-sm text-muted-foreground">{d.term}</dt>
                  <dd className="font-medium">{d.value}</dd>
                </div>
              ))}
            </dl>
          )}

          {book.genres.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {book.genres.map((genre) => (
                <Link key={genre.id} href={`/books?genre_id=${genre.id}`}>
                  <Badge variant="outline" className="hover:bg-muted">
                    {genre.name}
                  </Badge>
                </Link>
              ))}
            </div>
          )}

          <div className="space-y-8 pt-4">
            <BookAdaptations
              items={adaptationItems}
              intro={<AdaptationVotePrompt bookId={book.id} slug={book.slug} />}
              renderExtra={(adaptation) => (
                <AdaptationVerdict bookId={book.id} screenWorkId={adaptation.id} />
              )}
            />
            <SimilarBooks bookId={book.id} />
            <ReviewsSection bookId={book.id} slug={book.slug} />
          </div>
        </div>
      </div>
    </main>
  );
}
