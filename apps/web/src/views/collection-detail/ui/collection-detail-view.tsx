'use client';

import { AuthorNameLink } from '@/entities/author';
import {
  COLLECTION_PAGE_SIZE,
  MovementBadge,
  SourceLine,
  adaptationLine,
  autoCaption,
  useCuratedCollection,
} from '@/entities/curated-collection';
import { ApiError } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { isHotlinkCoverUrl } from '@/shared/lib/hotlink-cover';

interface CollectionDetailViewProps {
  slug: string;
}

function parsePage(value: string | null): number {
  const n = parseInt(value ?? '', 10);
  return Number.isNaN(n) || n < 1 ? 1 : n;
}

export function CollectionDetailView({ slug }: CollectionDetailViewProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const page = parsePage(searchParams.get('page'));
  const { data, isLoading, error } = useCuratedCollection(slug, page);

  if (isLoading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
      </main>
    );
  }

  if (error || !data) {
    const notFound = error instanceof ApiError && error.status === 404;
    return (
      <main className="container mx-auto px-4 py-8">
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {notFound ? 'Подборка не найдена' : 'Не удалось загрузить подборку'}
        </div>
        <Link href="/top" className="mt-4 inline-block text-sm text-primary hover:underline">
          Все подборки
        </Link>
      </main>
    );
  }

  const { collection, items, total } = data;
  const totalPages = Math.max(1, Math.ceil(total / COLLECTION_PAGE_SIZE));
  const goToPage = (target: number) => {
    router.push((target > 1 ? `/top/${slug}?page=${target}` : `/top/${slug}`) as any);
  };

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <Link href="/top" className="text-sm text-primary hover:underline">
        ← Все подборки
      </Link>
      <h1 className="mt-2 font-display text-2xl font-semibold tracking-tight md:text-3xl">
        {collection.title}
      </h1>
      {collection.description && <p className="mt-2 text-muted-foreground">{collection.description}</p>}
      <SourceLine
        source={collection.source}
        url={collection.source_url}
        className="mt-2 text-sm text-muted-foreground"
      />
      {collection.kind === 'auto' && collection.rule && (
        <p className="mt-1 text-sm text-muted-foreground">
          {autoCaption(collection.rule, collection.computed_at)}
        </p>
      )}
      {collection.genre && (
        <p className="mt-1 text-sm">
          Жанр:{' '}
          <Link
            href={`/books?genre_id=${collection.genre.id}` as any}
            className="text-primary hover:underline"
          >
            {collection.genre.name}
          </Link>
        </p>
      )}

      {items.length === 0 ? (
        <p className="mt-8 text-muted-foreground">В этой подборке пока нет книг</p>
      ) : (
        <>
          <ol className="mt-8 divide-y">
            {items.map(({ place, movement, adaptation, book }) => {
              const verdictLine = collection.rule ? adaptationLine(collection.rule.metric, adaptation) : null;
              return (
                <li key={book.id} className="flex items-center gap-4 py-4">
                  <div className="flex w-14 shrink-0 flex-col items-end">
                    <span
                      className="font-display text-3xl font-semibold text-muted-foreground"
                      aria-label={`Место ${place}`}
                    >
                      {place}
                    </span>
                    <MovementBadge movement={movement} />
                  </div>
                  <Link href={`/books/${book.slug}`} className="shrink-0">
                    <div className="relative h-24 w-16 overflow-hidden rounded-md bg-muted">
                      {book.coverUrl && (
                        <Image src={book.coverUrl} unoptimized={isHotlinkCoverUrl(book.coverUrl)} alt={book.title} fill className="object-cover" sizes="64px" />
                      )}
                    </div>
                  </Link>
                  <div className="min-w-0">
                    <Link href={`/books/${book.slug}`} className="font-semibold leading-tight hover:underline">
                      {book.title}
                    </Link>
                    <p className="text-sm text-muted-foreground">
                      {book.authors.map((a, i) => (
                        <span key={a.id}>
                          {i > 0 && ', '}
                          <AuthorNameLink author={a} linkClassName="hover:underline" />
                        </span>
                      ))}
                    </p>
                    <p className="text-sm text-muted-foreground">
                      {book.ratingsCount > 0 ? `★ ${book.rating.toFixed(1)} (${book.ratingsCount})` : 'Нет оценок'}
                    </p>
                    {verdictLine && <p className="text-sm text-muted-foreground">{verdictLine}</p>}
                  </div>
                </li>
              );
            })}
          </ol>

          {totalPages > 1 && (
            <div className="mt-8 flex items-center justify-center gap-4">
              <Button variant="outline" onClick={() => goToPage(page - 1)} disabled={page <= 1}>
                Назад
              </Button>
              <span className="text-sm text-muted-foreground">
                Страница {page} из {totalPages}
              </span>
              <Button variant="outline" onClick={() => goToPage(page + 1)} disabled={page >= totalPages}>
                Вперёд
              </Button>
            </div>
          )}
        </>
      )}
    </main>
  );
}
