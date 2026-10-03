'use client';

import { BookCard, booksApi } from '@/entities/book';
import { useQuery } from '@tanstack/react-query';

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
        <ul className="mt-4 flex gap-4 overflow-hidden pb-2">
          {Array.from({ length: SKELETON_COUNT }, (_, i) => (
            <li key={i} className="w-40 shrink-0 sm:w-44" data-testid="similar-books-skeleton">
              <div className="aspect-[2/3] animate-pulse rounded-xl bg-muted" />
              <div className="mt-3 h-4 w-3/4 animate-pulse rounded bg-muted" />
              <div className="mt-2 h-3 w-1/2 animate-pulse rounded bg-muted" />
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
      <ul className="mt-4 flex snap-x snap-mandatory gap-4 overflow-x-auto pb-2">
        {data.items.map((book) => (
          <li key={book.id} className="w-40 shrink-0 snap-start sm:w-44">
            <BookCard book={book} imageSizes="176px" />
          </li>
        ))}
      </ul>
    </section>
  );
}
