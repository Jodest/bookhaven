import type { CuratedCollection } from '@bookhaven/api-client';
import Image from 'next/image';
import Link from 'next/link';
import { formatBooksCount, formatSource } from '../lib/format-source';

interface CollectionCardProps {
  collection: CuratedCollection;
  /** Heading level of the title; h3 when the card sits under a section h2. */
  headingLevel?: 'h2' | 'h3';
}

/** Index card of /top: title, up to 4 covers, «{n} книг» and the source line. */
export function CollectionCard({ collection, headingLevel = 'h2' }: CollectionCardProps) {
  const source = formatSource(collection.source);
  const Heading = headingLevel;
  const covers = collection.cover_urls.slice(0, 4);

  return (
    <Link
      href={`/top/${collection.slug}`}
      className="group block rounded-xl border bg-card p-4 transition hover:-translate-y-0.5 hover:shadow-card"
    >
      <div className="grid grid-cols-4 gap-2" aria-hidden={covers.length === 0}>
        {covers.map((url) => (
          <div key={url} className="relative aspect-[2/3] overflow-hidden rounded-md bg-muted">
            <Image src={url} alt="" fill className="object-cover" sizes="100px" />
          </div>
        ))}
      </div>
      <Heading className="mt-3 font-display text-lg font-semibold leading-tight">{collection.title}</Heading>
      <p className="mt-1 text-sm text-muted-foreground">{formatBooksCount(collection.shown_count)}</p>
      {source && <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{source}</p>}
    </Link>
  );
}
