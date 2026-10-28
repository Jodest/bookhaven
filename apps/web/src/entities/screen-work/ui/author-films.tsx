import type { AuthorFilm } from '@bookhaven/api-client';
import { ScreenWorkCard } from './screen-work-card';

interface AuthorFilmsProps {
  items: AuthorFilm[];
}

/** «Фильмы об авторе» block of the author page: compact cards. Renders nothing for an empty list. */
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
      <ul className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
        {items.map((item) => (
          <li key={item.id} className="min-w-0">
            <ScreenWorkCard
              title={item.title}
              kind={item.kind}
              year={item.year}
              poster={item.poster}
              documentary={item.relation === 'documentary'}
              director={item.director}
              kinopoiskUrl={item.kinopoisk_url}
              imdbUrl={item.imdb_url}
            />
          </li>
        ))}
      </ul>
    </section>
  );
}
