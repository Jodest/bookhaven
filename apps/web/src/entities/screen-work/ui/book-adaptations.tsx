import type { BookAdaptation } from '@bookhaven/api-client';
import type { ReactNode } from 'react';
import { CollapsibleCardList } from './collapsible-card-list';
import { ScreenWorkCard } from './screen-work-card';

interface BookAdaptationsProps {
  items: BookAdaptation[];
  /** Rendered under the metadata of each card (the book page puts the verdict here). */
  renderExtra?: (item: BookAdaptation) => ReactNode;
  /** Rendered once under the heading (the book page puts the voting prompt here). */
  intro?: ReactNode;
}

/** «Экранизации» block of the book page: compact cards, first 4 shown. Renders nothing for an empty list. */
export function BookAdaptations({ items, renderExtra, intro }: BookAdaptationsProps) {
  if (items.length === 0) return null;

  return (
    <section id="adaptations" aria-labelledby="book-adaptations-title" className="scroll-mt-24">
      <h2 id="book-adaptations-title" className="font-display text-xl font-semibold">
        Экранизации
      </h2>
      {intro}
      <div className="mt-3">
        <CollapsibleCardList total={items.length} className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {items.map((item) => (
            <li key={item.id} tabIndex={-1} className="min-w-0 outline-none">
              <ScreenWorkCard
                title={item.title}
                kind={item.kind}
                year={item.year}
                poster={item.poster}
                director={item.director}
                kinopoiskUrl={item.kinopoisk_url}
                imdbUrl={item.imdb_url}
              >
                {renderExtra?.(item)}
              </ScreenWorkCard>
            </li>
          ))}
        </CollapsibleCardList>
      </div>
    </section>
  );
}
