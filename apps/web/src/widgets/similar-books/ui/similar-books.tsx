'use client';

import { booksApi } from '@/entities/book';
import { useQuery } from '@tanstack/react-query';
import Image from 'next/image';
import Link from 'next/link';

const SIMILAR_LIMIT = 8;
const SKELETON_COUNT = 4;
const HEADING_ID = 'similar-books-heading';

interface SimilarBooksProps {
  bookId: string;
  className?: string;
}

/**
 * "Похожие книги" row on the book page (see docs/features/similar-books.md).
 * A secondary block: renders nothing on an empty result or an error.
 */
export function SimilarBooks({ bookId, className }: SimilarBooksProps) {
  const { data, isPending, isError } = useQuery({
    queryKey: ['books', bookId, 'similar'],
    queryFn: () => booksApi.similar(bookId, { limit: SIMILAR_LIMIT }),
    staleTime: 5 * 60_000,
  });

  if (isPending) {
    return (
      <section aria-busy="true" aria-labelledby={HEADING_ID} className={className}>
        <h2 id={HEADING_ID} className="font-display text-xl font-semibold">
          Похожие книги
        </h2>
        <ul className="mt-3 flex gap-3 overflow-hidden pb-2">
          {Array.from({ length: SKELETON_COUNT }, (_, i) => (
            <li key={i} className="w-24 shrink-0 sm:w-28" data-testid="similar-books-skeleton">
              <div className="aspect-[2/3] animate-pulse rounded-md bg-muted" />
              <div className="mt-2 h-4 w-3/4 animate-pulse rounded bg-muted" />
            </li>
          ))}
        </ul>
      </section>
    );
  }

  if (isError || !data || data.items.length === 0) {
    return null;
  }

  return (
    <section aria-labelledby={HEADING_ID} className={className}>
      <h2 id={HEADING_ID} className="font-display text-xl font-semibold">
        Похожие книги
      </h2>
      <ul
        tabIndex={0}
        aria-labelledby={HEADING_ID}
        className="mt-3 flex snap-x gap-3 overflow-x-auto rounded-md pb-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {data.items.map((book) => (
          <li key={book.id} className="w-24 shrink-0 snap-start sm:w-28">
            <Link href={`/books/${book.slug}`} className="group block">
              <div className="relative aspect-[2/3] overflow-hidden rounded-md border bg-muted">
                {book.coverUrl ? (
                  <Image
                    src={book.coverUrl}
                    alt={book.title}
                    fill
                    sizes="112px"
                    className="object-cover transition-transform group-hover:scale-105"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center p-1 text-center text-xs text-muted-foreground">
                    Нет обложки
                  </div>
                )}
              </div>
              <p className="mt-2 line-clamp-2 text-sm font-medium leading-tight">{book.title}</p>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
