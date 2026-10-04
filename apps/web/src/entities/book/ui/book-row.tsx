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
 * Titled horizontal snap-scroll row of BookCards (same markup as the
 * similar-books row). Renders nothing for an empty list.
 */
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
      <ul className="mt-4 flex snap-x snap-mandatory gap-4 overflow-x-auto pb-2">
        {books.map((book) => (
          <li key={book.id} className="w-40 shrink-0 snap-start sm:w-44">
            <BookCard book={book} imageSizes="176px" />
          </li>
        ))}
      </ul>
    </section>
  );
}
