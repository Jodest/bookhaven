import { Book } from '@bookhaven/api-client';
import type { Route } from 'next';
import Link from 'next/link';
import { BookCard } from './book-card';

interface BookRowProps {
  title: string;
  books: Book[];
  /** Optional "see all" link next to the heading. */
  href?: Route;
  linkLabel?: string;
  className?: string;
}

/**
 * Titled one-row grid of BookCards without horizontal scroll. Lives in the
 * home main column (next to the sidebar at lg+): 2 columns (2 rows = 4 books)
 * on phones, then 3 / 4 / 3 / 4 / 5 columns in a single row from sm / md / lg /
 * xl / 2xl; surplus books are hidden by CSS. Renders nothing for an empty list.
 */
// Per-book visibility by index (columns: base 2 x 2 rows = 4, sm 3, md 4, lg 3, xl 4, 2xl 5).
function visibilityClass(index: number): string {
  if (index < 3) return '';
  if (index === 3) return 'sm:hidden md:block lg:hidden xl:block';
  if (index === 4) return 'hidden 2xl:block';
  return 'hidden';
}

export function BookRow({ title, books, href, linkLabel, className }: BookRowProps) {
  if (books.length === 0) {
    return null;
  }

  return (
    <section aria-label={title} className={className}>
      <div className="flex items-baseline justify-between gap-4">
        <h2 className="font-display text-xl font-semibold md:text-2xl">{title}</h2>
        {href && linkLabel && (
          <Link href={href} className="shrink-0 text-sm font-medium text-primary hover:underline">
            {linkLabel}
          </Link>
        )}
      </div>
      <ul className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5">
        {books.map((book, i) => (
          <li key={book.id} className={`min-w-0 ${visibilityClass(i)}`}>
            <BookCard book={book} imageSizes="(min-width: 1536px) 14vw, (min-width: 1280px) 17vw, (min-width: 1024px) 18vw, (min-width: 768px) 22vw, 45vw" />
          </li>
        ))}
      </ul>
    </section>
  );
}
