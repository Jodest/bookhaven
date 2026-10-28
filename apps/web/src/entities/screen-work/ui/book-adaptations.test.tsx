import type { BookAdaptation } from '@bookhaven/api-client';
import { fireEvent, render, screen, within } from '@testing-library/react';
import type { ImgHTMLAttributes } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { BookAdaptations } from './book-adaptations';

vi.mock('next/image', () => ({
  default: ({
    fill: _fill,
    unoptimized,
    ...props
  }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; unoptimized?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} data-unoptimized={unoptimized ? 'true' : 'false'} />
  ),
}));

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
    poster: null,
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

  it('renders a visible heading and the adaptations anchor', () => {
    const { container } = render(<BookAdaptations items={[item()]} />);
    expect(screen.getByRole('heading', { name: 'Экранизации' })).not.toHaveClass('sr-only');
    expect(container.querySelector('section#adaptations')).not.toBeNull();
  });

  it('has no show-all button for 4 items', () => {
    const items = [1, 2, 3, 4].map((n) => item({ id: `w${n}`, title: `Фильм ${n}` }));
    render(<BookAdaptations items={items} />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('keeps all 6 cards in the output and reveals them with the show-all button', () => {
    const items = [1, 2, 3, 4, 5, 6].map((n) => item({ id: `w${n}`, title: `Фильм ${n}` }));
    render(<BookAdaptations items={items} />);
    expect(screen.getAllByRole('listitem')).toHaveLength(6);
    const button = screen.getByRole('button', { name: 'Показать все (6)' });
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByRole('list')).toHaveAttribute('data-expanded', 'false');
    expect(screen.getByRole('list')).toHaveClass('[&>li:nth-child(n+5)]:data-[expanded=false]:hidden');
    expect(button).toHaveAttribute('aria-controls', screen.getByRole('list').id);

    fireEvent.click(button);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByRole('list')).toHaveAttribute('data-expanded', 'true');
    expect(screen.getAllByRole('listitem')[4]).toHaveFocus();
  });

  it('shows a placeholder with the type icon and the year when there is no poster', () => {
    render(<BookAdaptations items={[item({ year: 1975 }), item({ id: 'w2', title: 'Без года', year: null })]} />);
    const [first, second] = screen.getAllByRole('listitem');
    const placeholder = within(first).getByTestId('poster-placeholder');
    expect(placeholder).toHaveTextContent('1975');
    expect(placeholder.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(within(second).getByTestId('poster-placeholder')).not.toHaveTextContent(/\d/);
  });

  it('shows the poster image with the title as alt text', () => {
    render(
      <BookAdaptations
        items={[
          item({
            poster: { url: '/posters/ab/abc.jpg', source: 'manual', author: null, license: null, license_url: null, source_url: null },
          }),
          item({
            id: 'w2',
            title: 'Мастер (фильм)',
            poster: {
              url: 'https://image.tmdb.org/t/p/w342/a.jpg',
              source: 'tmdb',
              author: null,
              license: null,
              license_url: null,
              source_url: null,
            },
          }),
        ]}
      />,
    );
    const [first, second] = screen.getAllByRole('img');
    expect(first).toHaveAttribute('alt', 'Мастер и Маргарита');
    expect(first).toHaveAttribute('data-unoptimized', 'false');
    expect(second).toHaveAttribute('data-unoptimized', 'true');
    expect(screen.queryByText(/Постер:/)).not.toBeInTheDocument();
    expect(screen.queryByTestId('poster-placeholder')).not.toBeInTheDocument();
  });

  it('falls back to the placeholder when the image fails to load', () => {
    render(
      <BookAdaptations
        items={[
          item({
            year: 1988,
            poster: { url: 'https://upload.wikimedia.org/x.jpg', source: 'tmdb', author: null, license: null, license_url: null, source_url: null },
          }),
        ]}
      />,
    );
    fireEvent.error(screen.getByRole('img'));
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(screen.getByTestId('poster-placeholder')).toHaveTextContent('1988');
  });

  it('credits a Commons poster under the image', () => {
    render(
      <BookAdaptations
        items={[
          item({
            poster: {
              url: 'https://upload.wikimedia.org/x.jpg',
              source: 'commons',
              author: 'Jane Doe',
              license: 'CC BY-SA 4.0',
              license_url: 'https://creativecommons.org/licenses/by-sa/4.0/',
              source_url: 'https://commons.wikimedia.org/wiki/File:X.jpg',
            },
          }),
        ]}
      />,
    );
    expect(screen.getByText(/Постер: Jane Doe,/)).toBeInTheDocument();
    const licence = screen.getByRole('link', { name: 'CC BY-SA 4.0' });
    expect(licence).toHaveAttribute('href', 'https://creativecommons.org/licenses/by-sa/4.0/');
    expect(licence).toHaveAttribute('target', '_blank');
    const file = screen.getByRole('link', { name: 'Wikimedia Commons' });
    expect(file).toHaveAttribute('href', 'https://commons.wikimedia.org/wiki/File:X.jpg');
    expect(file).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  it('leaves out an unknown author in the credit', () => {
    render(
      <BookAdaptations
        items={[
          item({
            poster: {
              url: 'https://upload.wikimedia.org/x.jpg',
              source: 'commons',
              author: null,
              license: 'CC0',
              license_url: null,
              source_url: 'https://commons.wikimedia.org/wiki/File:X.jpg',
            },
          }),
        ]}
      />,
    );
    expect(screen.getByText(/^Постер: CC0,/)).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'CC0' })).not.toBeInTheDocument();
  });

  it('renders nothing for an empty list', () => {
    const { container } = render(<BookAdaptations items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
