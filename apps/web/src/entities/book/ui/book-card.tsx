import { Badge } from '@bookhaven/ui';
import { Book } from '@bookhaven/api-client';
import { Star } from 'lucide-react';
import Image from 'next/image';
import Link from 'next/link';
import { isHotlinkCoverUrl } from '@/shared/lib/hotlink-cover';

interface BookCardProps {
  book: Book;
  /** `sizes` for the cover image; defaults to the responsive grid value. */
  imageSizes?: string;
}

const DEFAULT_IMAGE_SIZES = '(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 25vw';

/**
 * Cover-first card used for book grids (home, catalog, author detail) and
 * the similar-books row.
 * Presentational only — no data fetching, no behavior beyond navigation.
 */
export function BookCard({ book, imageSizes = DEFAULT_IMAGE_SIZES }: BookCardProps) {
  return (
    <article className="group overflow-hidden rounded-xl border bg-card text-card-foreground shadow-sm transition hover:-translate-y-0.5 hover:shadow-card">
      <Link href={`/books/${book.slug}`} className="block">
        <div className="relative aspect-[2/3] overflow-hidden bg-muted">
          {book.coverUrl ? (
            <Image
              src={book.coverUrl}
              unoptimized={isHotlinkCoverUrl(book.coverUrl)}
              alt={book.title}
              fill
              className="object-cover transition-transform group-hover:scale-105"
              sizes={imageSizes}
            />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              Нет обложки
            </div>
          )}
          {book.rating > 0 && (
            <div className="absolute left-2 top-2 inline-flex items-center gap-1 rounded-full bg-highlight px-2 py-0.5 text-xs font-semibold text-highlight-foreground shadow-sm">
              <Star className="h-3 w-3 fill-current" />
              {book.rating.toFixed(1)}
            </div>
          )}
        </div>
        <div className="p-4">
          <h3 className="line-clamp-2 text-base font-semibold leading-tight">{book.title}</h3>
          <p className="mt-1 line-clamp-1 text-sm text-muted-foreground">
            {book.authors.map((author) => author.name).join(', ')}
          </p>
          {book.genres.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1">
              {book.genres.slice(0, 2).map((genre) => (
                <Badge key={genre.id} variant="outline" className="font-normal">
                  {genre.name}
                </Badge>
              ))}
            </div>
          )}
        </div>
      </Link>
    </article>
  );
}
