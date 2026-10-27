import type { BookAdaptation } from '@bookhaven/api-client';
import type { ReactNode } from 'react';
import { screenWorkKindLabel } from '../lib/kind-label';

interface BookAdaptationsProps {
  items: BookAdaptation[];
  /** Rendered at the bottom of each card (the book page puts the verdict here). */
  renderExtra?: (item: BookAdaptation) => ReactNode;
  /** Rendered once under the heading (the book page puts the voting prompt here). */
  intro?: ReactNode;
}

/** «Экранизации» block of the book page. Renders nothing for an empty list. */
export function BookAdaptations({ items, renderExtra, intro }: BookAdaptationsProps) {
  if (items.length === 0) return null;

  return (
    <section aria-labelledby="book-adaptations-title">
      <h2 id="book-adaptations-title" className="font-display text-xl font-semibold">
        Экранизации
      </h2>
      {intro}
      <ul className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {items.map((item) => (
          <li key={item.id} className="rounded-lg border p-3">
            <p className="font-medium">{item.title}</p>
            <p className="mt-1 flex flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground">
              {[
                item.year ? <span key="year">{item.year}</span> : null,
                <span key="kind">{screenWorkKindLabel(item.kind)}</span>,
                item.director ? <span key="director">Режиссёр: {item.director}</span> : null,
                item.kinopoisk_url ? (
                  <a
                    key="kp"
                    href={item.kinopoisk_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    Кинопоиск
                  </a>
                ) : null,
                item.imdb_url ? (
                  <a
                    key="imdb"
                    href={item.imdb_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    IMDb
                  </a>
                ) : null,
              ]
                .filter(Boolean)
                .flatMap((part, i) => (i === 0 ? [part] : [<span key={`sep-${i}`} aria-hidden="true">·</span>, part]))}
            </p>
            {renderExtra?.(item)}
          </li>
        ))}
      </ul>
    </section>
  );
}
