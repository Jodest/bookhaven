import { render, screen } from '@testing-library/react';
import type { Book, HomeCalendar } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { LiteraryCalendar } from './literary-calendar';

function book(id: string, title: string): Book {
  return {
    id,
    title,
    slug: id,
    description: null,
    coverUrl: null,
    isbn: null,
    publishedYear: 1926,
    pageCount: null,
    language: '',
    rating: 0,
    ratingsCount: 0,
    reviewsCount: 0,
    authors: [],
    genres: [],
    createdAt: '',
    updatedAt: '',
  };
}

const author = (id: string, name: string) => ({
  id,
  name,
  slug: id,
  photoUrl: null,
  birthDate: '1814-10-15',
  deathDate: '1841-07-27',
});

function calendar(overrides: Partial<HomeCalendar> = {}): HomeCalendar {
  return {
    date: '2026-10-04',
    isToday: true,
    events: [
      { type: 'birth', years: 212, author: author('a1', 'Михаил Лермонтов'), books: [book('b1', 'Герой нашего времени')] },
      { type: 'death', years: 122, author: author('a2', 'Антон Чехов'), books: [] },
    ],
    anniversaries: [],
    ...overrides,
  };
}

describe('LiteraryCalendar', () => {
  it('renders nothing for null', () => {
    const { container } = render(<LiteraryCalendar calendar={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the today heading with birth and death wording', () => {
    render(<LiteraryCalendar calendar={calendar()} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Литературный календарь · 4 октября');
    expect(screen.getByText('Родился 212 лет назад')).toBeInTheDocument();
    expect(screen.getByText('Умер 122 года назад')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Михаил Лермонтов' })).toHaveAttribute('href', '/authors/a1');
    expect(screen.getByRole('link', { name: 'Герой нашего времени' })).toHaveAttribute('href', '/books/b1');
  });

  it('shows the "Скоро" heading for a lookahead day', () => {
    render(<LiteraryCalendar calendar={calendar({ date: '2026-10-15', isToday: false })} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Скоро в календаре · 15 октября');
  });

  it('renders the anniversaries list', () => {
    render(
      <LiteraryCalendar
        calendar={calendar({
          events: [],
          anniversaries: [{ years: 100, book: book('b2', 'Мастер и Маргарита') }],
        })}
      />
    );
    expect(screen.getByText('Юбилеи года')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '«Мастер и Маргарита»' })).toHaveAttribute('href', '/books/b2');
    expect(screen.getByText(/— 100 лет/)).toBeInTheDocument();
  });
});
