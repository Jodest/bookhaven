'use client';

import type { Book } from '@bookhaven/api-client';
import { formatRatingSummary } from '@/shared/lib/format-date-ru';
import { Star } from 'lucide-react';
import Image from 'next/image';
import Link from 'next/link';
import { usePersonalHome } from '../model/use-personal-home';
import { CardSkeleton } from './row-skeleton';

interface WantToReadCardProps {
  shelfId: string;
  total: number;
  book: Book;
}

/**
 * One book from the user's "Хочу прочитать" shelf (picked server-side per
 * user and day), shown in the home main column for logged-in users: skeleton
 * while loading, nothing for guests, while the session resolves, when the
 * shelf is empty or on error.
 */
export function WantToReadPick() {
  const { enabled, query } = usePersonalHome();

  if (!enabled) {
    return null;
  }
  if (query.isPending) {
    return <CardSkeleton label="Из вашего «Хочу прочитать»" />;
  }
  const pick = query.data?.wantToReadPick;
  if (!pick?.book) {
    return null;
  }
  return <WantToReadCard shelfId={pick.shelfId} total={pick.total} book={pick.book} />;
}

function WantToReadCard({ shelfId, total, book }: WantToReadCardProps) {
  const rating = formatRatingSummary(book.rating, book.ratingsCount);

  return (
    <section
      aria-labelledby="home-want-to-read-heading"
      className="rounded-2xl border bg-card p-5 text-card-foreground shadow-sm"
    >
      <div className="flex items-baseline justify-between gap-4">
        <h2 id="home-want-to-read-heading" className="font-display text-xl font-semibold">
          Из вашего «Хочу прочитать»
        </h2>
        <Link
          href={`/shelves/${shelfId}`}
          className="shrink-0 text-sm font-medium text-primary hover:underline"
        >
          Вся полка ({total})
        </Link>
      </div>
      <div className="mt-4 flex gap-4">
        <Link
          href={`/books/${book.slug}`}
          aria-hidden="true"
          tabIndex={-1}
          className="relative block aspect-[2/3] w-20 shrink-0 overflow-hidden rounded-lg bg-muted shadow-sm"
        >
          {book.coverUrl ? (
            <Image src={book.coverUrl} alt="" fill className="object-cover" sizes="80px" />
          ) : (
            <span className="flex h-full items-center justify-center p-1 text-center text-xs text-muted-foreground">
              Нет обложки
            </span>
          )}
        </Link>
        <div className="min-w-0">
          <h3 className="font-display text-lg font-semibold leading-tight">
            <Link href={`/books/${book.slug}`} className="hover:underline">
              {book.title}
            </Link>
          </h3>
          {book.authors.length > 0 && (
            <p className="mt-1 text-sm text-muted-foreground">
              {book.authors.map((a) => a.name).join(', ')}
            </p>
          )}
          {rating && (
            <p className="mt-2 inline-flex items-center gap-1 text-sm font-medium">
              <Star className="h-4 w-4 fill-current text-highlight" aria-hidden="true" />
              {rating}
            </p>
          )}
        </div>
      </div>
    </section>
  );
}
