import { render, screen } from '@testing-library/react';
import type { ImgHTMLAttributes } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { ScreenWorkCard } from './screen-work-card';

vi.mock('next/image', () => ({
  default: ({ fill: _fill, unoptimized: _u, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; unoptimized?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} />
  ),
}));

describe('ScreenWorkCard', () => {
  it('renders the title, the metadata line with separators and the links', () => {
    render(
      <ScreenWorkCard
        title="Толкин"
        kind="film"
        year={2019}
        poster={null}
        documentary
        director="Доме Каруко"
        kinopoiskUrl="https://www.kinopoisk.ru/film/1/"
        imdbUrl="https://www.imdb.com/title/tt1/"
      >
        <p>extra</p>
      </ScreenWorkCard>,
    );
    expect(screen.getByText('Толкин')).toBeInTheDocument();
    expect(screen.getByText('Режиссёр: Доме Каруко').closest('p')).toHaveTextContent(
      '2019·Фильм·Документальный·Режиссёр: Доме Каруко·Кинопоиск·IMDb',
    );
    const kp = screen.getByRole('link', { name: 'Кинопоиск' });
    expect(kp).toHaveAttribute('href', 'https://www.kinopoisk.ru/film/1/');
    expect(kp).toHaveAttribute('target', '_blank');
    expect(kp).toHaveAttribute('rel', expect.stringContaining('noopener'));
    expect(screen.getByRole('link', { name: 'IMDb' })).toHaveAttribute('href', 'https://www.imdb.com/title/tt1/');
    expect(screen.getByText('extra')).toBeInTheDocument();
  });

  it('shows the placeholder with the year when there is no poster', () => {
    render(<ScreenWorkCard title="Без постера" kind="series" year={1999} poster={null} />);
    expect(screen.getByTestId('poster-placeholder')).toHaveTextContent('1999');
  });

  it('shows the image with the title as alt and credits a Commons poster', () => {
    render(
      <ScreenWorkCard
        title="Фильм"
        kind="film"
        year={null}
        poster={{
          url: 'https://upload.wikimedia.org/x.jpg',
          source: 'commons',
          author: 'Jane Doe',
          license: 'CC0',
          license_url: null,
          source_url: 'https://commons.wikimedia.org/wiki/File:X.jpg',
        }}
      />,
    );
    expect(screen.getByRole('img')).toHaveAttribute('alt', 'Фильм');
    expect(screen.getByText(/Постер: Jane Doe, CC0,/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Wikimedia Commons' })).toHaveAttribute(
      'href',
      'https://commons.wikimedia.org/wiki/File:X.jpg',
    );
  });
});
