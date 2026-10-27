import type { BookAdaptation } from '@bookhaven/api-client';
import type { ReactNode } from 'react';
import { screenWorkKindLabel } from '../lib/kind-label';
import { PosterAttribution } from './poster-attribution';
import { ScreenWorkPoster } from './screen-work-poster';

interface BookAdaptationsProps {
  items: BookAdaptation[];
  /** Rendered at the bottom of each card (the book page puts the verdict here). */
  renderExtra?: (item: BookAdaptation) => ReactNode;
  /** Rendered once under the heading (the book page puts the voting prompt here). */
  intro?: ReactNode;
  /** Hide the heading visually (the book page shows it as a tab). */
  headingHidden?: boolean;
}

/** «Экранизации» block of the book page: poster cards. Renders nothing for an empty list. */
export function BookAdaptations({ items, renderExtra, intro, headingHidden }: BookAdaptationsProps) {
  if (items.length === 0) return null;

  return (
    <section id="adaptations" aria-labelledby="book-adaptations-title">
      <h2
        id="book-adaptations-title"
        className={headingHidden ? 'sr-only' : 'font-display text-xl font-semibold'}
      >
        Экранизации
      </h2>
      {intro}
      <ul className="mt-3 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {items.map((item) => (
          <li key={item.id} className="min-w-0">
            <ScreenWorkPoster title={item.title} kind={item.kind} year={item.year} poster={item.poster} />
            <PosterAttribution poster={item.poster} className="mt-1" />
            <p className="mt-2 font-medium">{item.title}</p>
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
