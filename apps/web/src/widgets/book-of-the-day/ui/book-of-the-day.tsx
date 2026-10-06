import { HomeBookOfTheDay } from '@bookhaven/api-client';
import { AuthorNameLink } from '@/entities/author';
import { ShelveBookButtons } from '@/features/shelve-book';
import { formatRatingSummary } from '@/shared/lib/format-date-ru';
import { Star } from 'lucide-react';
import Image from 'next/image';
import Link from 'next/link';

interface BookOfTheDayProps {
  data: HomeBookOfTheDay | null;
  className?: string;
}

/**
 * "Книга дня" card (see docs/features/home-page-content.md). The book is
 * picked server-side and is the same all day for every visitor. Renders
 * nothing without a book.
 */
export function BookOfTheDay({ data, className }: BookOfTheDayProps) {
  if (!data) {
    return null;
  }
  const { book, descriptionExcerpt } = data;
  const rating = formatRatingSummary(book.rating, book.ratingsCount);

  return (
    <section
      aria-labelledby="book-of-the-day-heading"
      className={`rounded-2xl border bg-card p-5 text-card-foreground shadow-sm md:p-6 ${className ?? ''}`}
    >
      <h2 id="book-of-the-day-heading" className="font-display text-xl font-semibold md:text-2xl">
        Книга дня
      </h2>
      <div className="mt-4 flex flex-col gap-5 sm:flex-row">
        <Link
          href={`/books/${book.slug}`}
          aria-hidden="true"
          tabIndex={-1}
          className="relative mx-auto block aspect-[2/3] w-[200px] shrink-0 overflow-hidden rounded-xl bg-muted shadow-sm sm:mx-0 lg:w-[260px]"
        >
          {book.coverUrl ? (
            <Image
              src={book.coverUrl}
              alt=""
              fill
              priority
              className="object-cover"
              sizes="(max-width: 1024px) 200px, 260px"
            />
          ) : (
            <span className="flex h-full items-center justify-center text-sm text-muted-foreground">
              Нет обложки
            </span>
          )}
        </Link>
        <div className="min-w-0 flex-1">
          <h3 className="font-display text-2xl font-semibold leading-tight">
            <Link href={`/books/${book.slug}`} className="hover:underline">
              {book.title}
            </Link>
          </h3>
          {book.authors.length > 0 && (
            <p className="mt-1 text-sm text-muted-foreground">
              {book.authors.map((author, i) => (
                <span key={author.id}>
                  {i > 0 && ', '}
                  <AuthorNameLink author={author} linkClassName="hover:underline" />
                </span>
              ))}
              {book.publishedYear ? ` · ${book.publishedYear}` : ''}
            </p>
          )}
          {rating && (
            <p className="mt-2 inline-flex items-center gap-1 text-sm font-medium">
              <Star className="h-4 w-4 fill-current text-highlight" aria-hidden="true" />
              {rating}
            </p>
          )}
          {descriptionExcerpt && (
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">{descriptionExcerpt}</p>
          )}
          <div className="mt-4 flex flex-wrap items-start gap-3">
            <ShelveBookButtons
              bookId={book.id}
              slug={book.slug}
              types={['want_to_read']}
              showCustomShelves={false}
              loginRedirect="/"
            />
            <Link
              href={`/books/${book.slug}`}
              className="inline-flex h-10 items-center text-sm font-medium text-primary hover:underline"
            >
              Подробнее
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
