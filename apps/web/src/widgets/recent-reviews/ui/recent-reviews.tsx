import { HomeReview } from '@bookhaven/api-client';
import { formatTimestampDayMonth } from '@/shared/lib/format-date-ru';
import { Star } from 'lucide-react';
import Image from 'next/image';
import Link from 'next/link';

interface RecentReviewsProps {
  reviews: HomeReview[];
  className?: string;
}

function Stars({ value }: { value: number }) {
  return (
    <div className="flex" role="img" aria-label={`Оценка ${value} из 5`}>
      {[1, 2, 3, 4, 5].map((i) => (
        <Star
          key={i}
          aria-hidden="true"
          className={`h-4 w-4 ${i <= value ? 'fill-current text-yellow-400' : 'text-muted-foreground/30'}`}
        />
      ))}
    </div>
  );
}

function Avatar({ name, url }: { name: string | null; url: string | null }) {
  return (
    <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-xs font-medium">
      {url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={url} alt="" className="h-full w-full object-cover" loading="lazy" />
      ) : (
        (name?.charAt(0).toUpperCase() ?? '?')
      )}
    </div>
  );
}

/**
 * "Свежие рецензии": read-only review excerpts linking to the book page.
 * ReviewCard is not reused because it carries owner/like actions. Renders
 * nothing for an empty list.
 */
export function RecentReviews({ reviews, className }: RecentReviewsProps) {
  if (reviews.length === 0) {
    return null;
  }

  return (
    <section aria-labelledby="recent-reviews-heading" className={className}>
      <h2 id="recent-reviews-heading" className="font-display text-xl font-semibold md:text-2xl">
        Свежие рецензии
      </h2>
      <ul className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-3">
        {reviews.map((review) => (
          <li key={review.id}>
            <article className="flex h-full flex-col rounded-xl border bg-card p-4 text-card-foreground shadow-sm">
              <div className="flex gap-3">
                <Link
                  href={`/books/${review.book.slug}`}
                  aria-hidden="true"
                  tabIndex={-1}
                  className="relative block aspect-[2/3] w-12 shrink-0 overflow-hidden rounded bg-muted"
                >
                  {review.book.coverUrl && (
                    <Image
                      src={review.book.coverUrl}
                      alt=""
                      fill
                      className="object-cover"
                      sizes="48px"
                    />
                  )}
                </Link>
                <div className="min-w-0">
                  <Link
                    href={`/books/${review.book.slug}`}
                    className="line-clamp-2 font-semibold leading-tight hover:underline"
                  >
                    {review.book.title}
                  </Link>
                  {review.book.authors.length > 0 && (
                    <p className="line-clamp-1 text-xs text-muted-foreground">
                      {review.book.authors.map((a) => a.name).join(', ')}
                    </p>
                  )}
                  <div className="mt-1">
                    <Stars value={review.rating} />
                  </div>
                </div>
              </div>
              <blockquote className="mt-3 flex-1 text-sm leading-relaxed">
                «{review.excerpt}»
              </blockquote>
              <Link
                href={`/books/${review.book.slug}`}
                className="mt-2 text-sm font-medium text-primary hover:underline"
              >
                Читать полностью
              </Link>
              <div className="mt-3 flex items-center gap-2 text-xs text-muted-foreground">
                <Avatar name={review.user.name} url={review.user.avatarUrl} />
                <span className="font-medium text-foreground">{review.user.name?.trim() || 'Аноним'}</span>
                <span aria-hidden="true">·</span>
                <time dateTime={review.createdAt}>{formatTimestampDayMonth(review.createdAt)}</time>
              </div>
            </article>
          </li>
        ))}
      </ul>
    </section>
  );
}
