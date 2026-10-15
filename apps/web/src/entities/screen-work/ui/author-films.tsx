import type { AuthorFilm } from '@bookhaven/api-client';
import { screenWorkKindLabel } from '../lib/kind-label';

interface AuthorFilmsProps {
  items: AuthorFilm[];
}

/** «Фильмы об авторе» block of the author page. Renders nothing for an empty list. */
export function AuthorFilms({ items }: AuthorFilmsProps) {
  if (items.length === 0) return null;

  return (
    <section className="py-12 md:py-16" aria-labelledby="author-films-title">
      <h2
        id="author-films-title"
        className="font-display mb-6 text-2xl font-semibold tracking-tight md:text-3xl"
      >
        Фильмы об авторе
      </h2>
      <ul className="space-y-3">
        {items.map((item) => (
          <li key={item.id} className="rounded-lg border p-3">
            <p className="font-medium">{item.title}</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {[
                item.year ? String(item.year) : null,
                screenWorkKindLabel(item.kind),
                item.relation === 'documentary' ? 'Документальный' : null,
              ]
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
