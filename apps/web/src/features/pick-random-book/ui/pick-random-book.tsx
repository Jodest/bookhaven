'use client';

import { Book } from '@bookhaven/api-client';
import { homeApi } from '@/entities/home';
import { formatRatingSummary } from '@/shared/lib/format-date-ru';
import { Button, Select } from '@bookhaven/ui';
import { useMutation } from '@tanstack/react-query';
import Image from 'next/image';
import Link from 'next/link';
import { useState } from 'react';
import { isHotlinkCoverUrl } from '@/shared/lib/hotlink-cover';

interface PickRandomBookProps {
  /** Genres for the filter, passed from the server-rendered page. */
  genres: { id: string; name: string }[];
  className?: string;
}

interface PickVars {
  genreId?: string;
  excludeId?: string;
}

/**
 * "Что почитать?" roulette: picks a random well-rated book (optionally in
 * a genre) via GET /home/random. Compact: lives in the narrow home sidebar,
 * so the select and button are always stacked. "Ещё вариант" excludes the book shown.
 */
export function PickRandomBook({ genres, className }: PickRandomBookProps) {
  const [genreId, setGenreId] = useState('');
  // The last result is kept in state so it stays visible while the next
  // pick is loading ("Ещё вариант").
  const [result, setResult] = useState<{ book: Book | null } | null>(null);
  const mutation = useMutation({
    mutationFn: (vars: PickVars) => homeApi.random(vars),
    onSuccess: (data) => setResult(data),
  });

  const pick = (excludeId?: string) => {
    mutation.mutate({ genreId: genreId || undefined, excludeId });
  };

  const book = result?.book ?? null;
  const showResult = result !== null && !mutation.isError;
  const rating = book ? formatRatingSummary(book.rating, book.ratingsCount) : null;

  return (
    <section
      aria-labelledby="pick-random-book-heading"
      className={`rounded-2xl border bg-card p-5 text-card-foreground shadow-sm ${className ?? ''}`}
    >
      <h2 id="pick-random-book-heading" className="font-display text-lg font-semibold">
        Что почитать?
      </h2>
      <p className="mt-1 text-sm text-muted-foreground">
        Подберём случайную книгу с хорошими оценками читателей.
      </p>

      <div className="mt-4 flex flex-col gap-3">
        <label htmlFor="pick-random-genre" className="sr-only">
          Жанр
        </label>
        <Select
          id="pick-random-genre"
          value={genreId}
          onChange={(e) => setGenreId(e.target.value)}
          options={[
            { value: '', label: 'Любой жанр' },
            ...genres.map((g) => ({ value: g.id, label: g.name })),
          ]}
        />
        <Button type="button" onClick={() => pick()} disabled={mutation.isPending}>
          {mutation.isPending ? 'Подбираем…' : 'Подобрать'}
        </Button>
      </div>

      <div aria-live="polite">
        {mutation.isError && (
          <p className="mt-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            Не удалось подобрать книгу. Попробуйте ещё раз.
          </p>
        )}

        {showResult && !book && (
          <p className="mt-4 text-sm text-muted-foreground">Не нашли подходящую книгу.</p>
        )}

        {showResult && book && (
          <div className="mt-4 flex gap-4 rounded-xl border p-3">
            <Link
              href={`/books/${book.slug}`}
              aria-hidden="true"
              tabIndex={-1}
              className="relative block aspect-[2/3] w-16 shrink-0 overflow-hidden rounded bg-muted"
            >
              {book.coverUrl && (
                <Image src={book.coverUrl} unoptimized={isHotlinkCoverUrl(book.coverUrl)} alt="" fill className="object-cover" sizes="64px" />
              )}
            </Link>
            <div className="min-w-0 flex-1">
              <Link href={`/books/${book.slug}`} className="font-semibold leading-tight hover:underline">
                {book.title}
              </Link>
              {book.authors.length > 0 && (
                <p className="line-clamp-1 text-sm text-muted-foreground">
                  {book.authors.map((a) => a.name).join(', ')}
                </p>
              )}
              {rating && <p className="mt-1 text-xs text-muted-foreground">{rating}</p>}
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="mt-2"
                onClick={() => pick(book.id)}
                disabled={mutation.isPending}
              >
                Ещё вариант
              </Button>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
