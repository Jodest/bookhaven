import Link from 'next/link';
import type { AdminBookStatus } from '@bookhaven/api-client';
import { BOOK_STATUS_LABELS } from './book-status-badge';

interface SampleBooksCellProps {
  booksTotal: number;
  booksVisible: number;
  sampleBooks: { id: string; title: string; status: AdminBookStatus }[];
}

/**
 * Books context of a moderated author/genre: "N (видимых M)" plus links to a
 * few sample books; non-published ones carry a lowercased status suffix.
 */
export function SampleBooksCell({ booksTotal, booksVisible, sampleBooks }: SampleBooksCellProps) {
  return (
    <>
      <div>
        {booksTotal} (видимых {booksVisible})
      </div>
      {sampleBooks.length > 0 && (
        <ul className="mt-1 space-y-0.5">
          {sampleBooks.map((b) => (
            <li key={b.id}>
              <Link href={`/catalog/books/${b.id}/edit` as any} className="text-primary underline">
                {b.title}
              </Link>
              {b.status !== 'published' && (
                <span className="ml-1 text-xs text-muted-foreground">
                  ({(BOOK_STATUS_LABELS[b.status] ?? b.status).toLowerCase()})
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
