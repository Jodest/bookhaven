import type { AuthorFilm } from '@bookhaven/api-client';
import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AuthorFilms } from './author-films';

function item(over: Partial<AuthorFilm> = {}): AuthorFilm {
  return {
    id: 'w1',
    title: 'Толкин',
    original_title: null,
    kind: 'film',
    relation: 'biopic',
    year: 2019,
    director: null,
    imdb_url: null,
    kinopoisk_url: null,
    poster: null,
    ...over,
  };
}

describe('AuthorFilms', () => {
  it('lists title, year, type, the documentary mark, director and links in a new tab', () => {
    render(
      <AuthorFilms
        items={[
          item({
            director: 'Доме Каруко',
            kinopoisk_url: 'https://www.kinopoisk.ru/film/1234/',
            imdb_url: 'https://www.imdb.com/title/tt3416828/',
          }),
          item({ id: 'w2', title: 'Волшебство слов', kind: 'tv_film', relation: 'documentary', year: 2011 }),
        ]}
      />,
    );

    expect(screen.getByRole('heading', { name: 'Фильмы об авторе' })).toBeInTheDocument();
    const [first, second] = screen.getAllByRole('listitem');
    expect(within(first).getByText('Толкин')).toBeInTheDocument();
    expect(within(first).getByText('Режиссёр: Доме Каруко').closest('p')).toHaveTextContent(
      '2019·Фильм·Режиссёр: Доме Каруко·Кинопоиск·IMDb',
    );
    expect(within(first).queryByText(/Документальный/)).not.toBeInTheDocument();
    expect(within(first).getByText('Режиссёр: Доме Каруко')).toBeInTheDocument();
    const kp = within(first).getByRole('link', { name: 'Кинопоиск' });
    expect(kp).toHaveAttribute('href', 'https://www.kinopoisk.ru/film/1234/');
    expect(kp).toHaveAttribute('target', '_blank');
    expect(kp).toHaveAttribute('rel', expect.stringContaining('noopener'));
    expect(within(first).getByRole('link', { name: 'IMDb' })).toHaveAttribute(
      'href',
      'https://www.imdb.com/title/tt3416828/',
    );

    expect(within(second).getByText('Документальный').closest('p')).toHaveTextContent('2011·Телефильм·Документальный');
    expect(within(second).queryByRole('link')).not.toBeInTheDocument();
  });

  it('shows the placeholder with the type icon and the year for a film without a poster', () => {
    render(<AuthorFilms items={[item({ id: 'w2', title: 'Волшебство слов', year: 2011, relation: 'documentary' })]} />);
    expect(screen.getByTestId('poster-placeholder')).toHaveTextContent('2011');
  });

  it('renders nothing for an empty list', () => {
    const { container } = render(<AuthorFilms items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
