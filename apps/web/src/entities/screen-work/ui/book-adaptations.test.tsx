import type { BookAdaptation } from '@bookhaven/api-client';
import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BookAdaptations } from './book-adaptations';

function item(over: Partial<BookAdaptation> = {}): BookAdaptation {
  return {
    id: 'w1',
    title: 'Мастер и Маргарита',
    original_title: null,
    kind: 'series',
    year: 2005,
    director: null,
    imdb_url: null,
    kinopoisk_url: null,
    ...over,
  };
}

describe('BookAdaptations', () => {
  it('lists title, year, type, director and links opening in a new tab', () => {
    render(
      <BookAdaptations
        items={[
          item({
            director: 'Владимир Бортко',
            kinopoisk_url: 'https://www.kinopoisk.ru/film/1234/',
            imdb_url: 'https://www.imdb.com/title/tt0000001/',
          }),
          item({ id: 'w2', title: 'Мастер (фильм)', kind: 'film', year: null }),
        ]}
      />,
    );

    expect(screen.getByRole('heading', { name: 'Экранизации' })).toBeInTheDocument();
    const [first, second] = screen.getAllByRole('listitem');
    expect(within(first).getByText('Мастер и Маргарита')).toBeInTheDocument();
    // Year, type, director and links share one line.
    const meta = within(first).getByText('Режиссёр: Владимир Бортко').closest('p');
    expect(meta).toHaveTextContent('2005·Сериал·Режиссёр: Владимир Бортко·Кинопоиск·IMDb');
    expect(within(first).getAllByRole('paragraph')).toHaveLength(2);
    const kp = within(first).getByRole('link', { name: 'Кинопоиск' });
    expect(kp).toHaveAttribute('href', 'https://www.kinopoisk.ru/film/1234/');
    expect(kp).toHaveAttribute('target', '_blank');
    expect(kp).toHaveAttribute('rel', expect.stringContaining('noopener'));
    expect(within(first).getByRole('link', { name: 'IMDb' })).toHaveAttribute(
      'href',
      'https://www.imdb.com/title/tt0000001/',
    );

    expect(within(second).getByText('Фильм')).toBeInTheDocument();
    expect(within(second).queryByRole('link')).not.toBeInTheDocument();
    expect(within(second).queryByText(/Режиссёр/)).not.toBeInTheDocument();
  });

  it('renders only the links the adaptation has', () => {
    render(<BookAdaptations items={[item({ imdb_url: 'https://www.imdb.com/title/tt1/' })]} />);
    expect(screen.queryByRole('link', { name: 'Кинопоиск' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'IMDb' })).toBeInTheDocument();
  });

  it('renders the extra slot at the bottom of each card', () => {
    render(
      <BookAdaptations
        items={[item(), item({ id: 'w2', title: 'Мастер (фильм)' })]}
        renderExtra={(a) => <p>{`extra ${a.id}`}</p>}
      />,
    );
    const [first, second] = screen.getAllByRole('listitem');
    expect(within(first).getByText('extra w1')).toBeInTheDocument();
    expect(within(second).getByText('extra w2')).toBeInTheDocument();
  });

  it('renders the intro once under the heading', () => {
    render(
      <BookAdaptations
        items={[item(), item({ id: 'w2', title: 'Мастер (фильм)' })]}
        intro={<p>intro text</p>}
      />,
    );
    expect(screen.getAllByText('intro text')).toHaveLength(1);
    const heading = screen.getByRole('heading', { name: 'Экранизации' });
    expect(heading.nextElementSibling).toHaveTextContent('intro text');
  });

  it('renders nothing for an empty list', () => {
    const { container } = render(<BookAdaptations items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
