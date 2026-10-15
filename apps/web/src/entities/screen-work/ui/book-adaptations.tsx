import type { BookAdaptation } from '@bookhaven/api-client';
import { screenWorkKindLabel } from '../lib/kind-label';

interface BookAdaptationsProps {
  items: BookAdaptation[];
}

/** «Экранизации» block of the book page. Renders nothing for an empty list. */
export function BookAdaptations({ items }: BookAdaptationsProps) {
  if (items.length === 0) return null;

  return (
    <section className="mt-8" aria-labelledby="book-adaptations-title">
      <h2 id="book-adaptations-title" className="font-display text-xl font-semibold">
        Экранизации
      </h2>
      <ul className="mt-3 space-y-3">
        {items.map((item) => (
          <li key={item.id} className="rounded-lg border p-3">
            <p className="font-medium">{item.title}</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {[item.year ? String(item.year) : null, screenWorkKindLabel(item.kind)]
                .filter(Boolean)
                .join(' · ')}
            </p>
            {item.director && (
              <p className="mt-1 text-sm text-muted-foreground">Режиссёр: {item.director}</p>
            )}
            {(item.kinopoisk_url || item.imdb_url) && (
              <p className="mt-2 flex gap-3 text-sm">
                {item.kinopoisk_url && (
                  <a
                    href={item.kinopoisk_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    Кинопоиск
                  </a>
                )}
                {item.imdb_url && (
                  <a
                    href={item.imdb_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    IMDb
                  </a>
                )}
              </p>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
