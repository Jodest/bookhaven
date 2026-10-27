import { fireEvent, render, screen } from '@testing-library/react';
import type { Book } from '@bookhaven/api-client';
import type { ImgHTMLAttributes, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { BookDetailView } from './book-detail-view';

const getBook = vi.fn();
const forBook = vi.fn();
const adaptationsForBook = vi.fn();

vi.mock('@/entities/book', () => ({
  booksApi: { get: (slug: string) => getBook(slug) },
}));

vi.mock('@/entities/curated-collection', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection')>()),
  collectionsApi: { forBook: (id: string) => forBook(id) },
}));

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  screenWorksApi: { forBook: (id: string) => adaptationsForBook(id) },
}));

vi.mock('next/image', () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: ({ fill: _fill, priority: _priority, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; priority?: boolean }) => <img {...props} />,
}));

vi.mock('next/navigation', () => ({
  notFound: vi.fn(),
  useSearchParams: () => new URLSearchParams(window.location.search),
}));
vi.mock('@/features/rate-book', () => ({ BookRating: () => null }));
vi.mock('@/features/shelve-book', () => ({ BookShelfActions: () => null }));
vi.mock('@/features/vote-adaptation', () => ({
  AdaptationVerdict: ({ bookId, screenWorkId }: { bookId: string; screenWorkId: string }) => (
    <p>{`verdict:${bookId}:${screenWorkId}`}</p>
  ),
  AdaptationVotePrompt: ({ bookId, slug }: { bookId: string; slug: string }) => (
    <p>{`prompt:${bookId}:${slug}`}</p>
  ),
}));
vi.mock('@/widgets/reviews-section', () => ({
  ReviewsSection: ({ headingHidden }: { headingHidden?: boolean }) => (
    <section>
      <h2>Рецензии</h2>
      <p>{`reviews-panel hidden-heading:${headingHidden ? 'yes' : 'no'}`}</p>
    </section>
  ),
}));
vi.mock('@/widgets/similar-books', () => ({
  SimilarBooks: ({ emptyFallback }: { emptyFallback?: ReactNode }) => (
    <section>
      <h2>Похожие книги</h2>
      {emptyFallback}
    </section>
  ),
}));

const author = {
  biography: null,
  photoUrl: null,
  birthDate: null,
  deathDate: null,
  createdAt: '',
  updatedAt: '',
};

const book: Book = {
  id: 'b1',
  title: 'Отцы и дети',
  slug: 'fathers-and-sons',
  description: null,
  coverUrl: null,
  isbn: null,
  publishedYear: 1862,
  pageCount: null,
  language: 'ru',
  rating: 0,
  ratingsCount: 0,
  reviewsCount: 0,
  authors: [
    { ...author, id: 'a1', name: 'Иван Тургенев', slug: 'ivan-turgenev' },
    { ...author, id: 'a2', name: 'Неизвестный автор', slug: null },
  ],
  genres: [],
  createdAt: '',
  updatedAt: '',
};

const adaptationItem = {
  id: 'w1',
  title: 'Отцы и дети (фильм)',
  original_title: null,
  kind: 'film',
  year: 1958,
  director: null,
  imdb_url: null,
  kinopoisk_url: null,
  poster: null,
};

beforeEach(() => {
  window.history.replaceState(null, '', '/');
  forBook.mockReset();
  forBook.mockResolvedValue({ items: [], total: 0 });
  adaptationsForBook.mockReset();
  adaptationsForBook.mockResolvedValue({ items: [], total: 0 });
});

describe('BookDetailView collection badges', () => {
  it('renders the places returned by the API', async () => {
    getBook.mockResolvedValue(book);
    forBook.mockResolvedValue({
      items: [{ place: 3, collection: { id: 'c1', slug: 'top-100', title: 'Топ-100', kind: 'manual', size: 100 } }],
      total: 1,
    });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(forBook).toHaveBeenCalledWith('b1');
    expect(screen.getByRole('link', { name: '№3 в „Топ-100“' })).toHaveAttribute('href', '/top/top-100');
  });

  it('still renders the page without badges when the places request fails', async () => {
    getBook.mockResolvedValue(book);
    forBook.mockRejectedValue(new Error('boom'));

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('heading', { level: 1, name: 'Отцы и дети' })).toBeInTheDocument();
    expect(screen.queryByLabelText('Места в подборках')).not.toBeInTheDocument();
    expect(screen.queryByText(/boom|ошибк/i)).not.toBeInTheDocument();
  });
});

describe('BookDetailView', () => {
  it('links published authors and renders non-published ones as plain text', async () => {
    getBook.mockResolvedValue(book);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('link', { name: 'Иван Тургенев' })).toHaveAttribute('href', '/authors/ivan-turgenev');
    expect(screen.getByText('Неизвестный автор')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Неизвестный автор' })).not.toBeInTheDocument();
    expect(document.querySelector('a[href="/authors/null"]')).toBeNull();
  });
});

describe('BookDetailView description', () => {
  const LABEL = 'Описание создано ИИ и может содержать неточности.';
  const AI_TEXT = 'Роман рассказывает о конфликте поколений в русской усадьбе.';

  it('shows an approved AI description with the label when the own description is blank', async () => {
    getBook.mockResolvedValue({ ...book, description: '  ', aiDescription: { text: AI_TEXT, generatedAt: '2026-10-13T12:00:00Z' } });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('heading', { name: 'Описание' })).toBeInTheDocument();
    expect(screen.getByText(AI_TEXT)).toBeInTheDocument();
    expect(screen.getByText(LABEL)).toBeInTheDocument();
  });

  it('the own description wins and has no label', async () => {
    getBook.mockResolvedValue({
      ...book,
      description: 'Описание из Open Library.',
      aiDescription: { text: AI_TEXT, generatedAt: '2026-10-13T12:00:00Z' },
    });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByText('Описание из Open Library.')).toBeInTheDocument();
    expect(screen.queryByText(AI_TEXT)).not.toBeInTheDocument();
    expect(screen.queryByText(LABEL)).not.toBeInTheDocument();
  });

  it('no description and a null AI description: no block', async () => {
    getBook.mockResolvedValue({ ...book, description: null, aiDescription: null });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.queryByRole('heading', { name: 'Описание' })).not.toBeInTheDocument();
    expect(screen.queryByText(LABEL)).not.toBeInTheDocument();
  });
});

describe('BookDetailView adaptations', () => {
  it('renders the adaptations block', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue({
      items: [
        {
          id: 'w1',
          title: 'Отцы и дети (фильм)',
          original_title: null,
          kind: 'film',
          year: 1958,
          director: 'Адольф Бергункер',
          imdb_url: null,
          kinopoisk_url: 'https://www.kinopoisk.ru/film/1/',
          poster: null,
        },
      ],
      total: 1,
    });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(adaptationsForBook).toHaveBeenCalledWith('b1');
    expect(screen.getByRole('heading', { name: 'Экранизации' })).toBeInTheDocument();
    expect(screen.getByText('Режиссёр: Адольф Бергункер')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Кинопоиск' })).toHaveAttribute('target', '_blank');
    // The verdict and voting controls are mounted under each adaptation.
    expect(screen.getByText('verdict:b1:w1')).toBeInTheDocument();
    // The voting prompt is mounted once for the whole block.
    expect(screen.getAllByText('prompt:b1:fathers-and-sons')).toHaveLength(1);
  });

  it('has no block for an empty list', async () => {
    getBook.mockResolvedValue(book);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.queryByRole('heading', { name: 'Экранизации' })).not.toBeInTheDocument();
  });

  it('renders the page without the block when the request fails', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockRejectedValue(new Error('boom'));

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('heading', { level: 1, name: 'Отцы и дети' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Экранизации' })).not.toBeInTheDocument();
    expect(screen.queryByText(/boom|ошибк/i)).not.toBeInTheDocument();
  });
});

describe('BookDetailView side requests', () => {
  it('starts both side requests before either resolves', async () => {
    getBook.mockResolvedValue(book);
    forBook.mockReturnValue(new Promise(() => {}));
    adaptationsForBook.mockReturnValue(new Promise(() => {}));

    // The view never resolves (both requests hang) but must have started both.
    void BookDetailView({ slug: 'fathers-and-sons' });
    await new Promise((r) => setTimeout(r, 10));

    expect(forBook).toHaveBeenCalledWith('b1');
    expect(adaptationsForBook).toHaveBeenCalledWith('b1');
  });

  it('renders the page when both side requests fail', async () => {
    getBook.mockResolvedValue(book);
    forBook.mockRejectedValue(new Error('boom'));
    adaptationsForBook.mockRejectedValue(new Error('boom'));

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('heading', { level: 1, name: 'Отцы и дети' })).toBeInTheDocument();
  });
});

describe('BookDetailView layout and summary', () => {
  it('has no rating badge on the cover', async () => {
    getBook.mockResolvedValue({ ...book, rating: 4.3, ratingsCount: 12, coverUrl: 'https://x.test/c.jpg' });

    const { container } = render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(container.querySelector('.bg-highlight')).toBeNull();
  });

  it('shows plural forms and a link to the reviews section', async () => {
    getBook.mockResolvedValue({ ...book, rating: 4.3, ratingsCount: 21, reviewsCount: 3 });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByText('21 оценка')).toBeInTheDocument();
    expect(screen.getByText('4.3')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '3 рецензии' })).toHaveAttribute('href', '?tab=reviews#book-tabs');
  });

  it('shows the empty summary for a book without ratings and reviews', async () => {
    getBook.mockResolvedValue(book);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByText('Пока нет оценок')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Написать рецензию' })).toHaveAttribute('href', '?tab=reviews#book-tabs');
  });

  it('keeps the panels with the voting prompt and verdicts inside the adaptations tab', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue({ items: [adaptationItem], total: 1 });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const panel = screen.getByRole('tabpanel', { name: 'Экранизации (1)' });
    expect(panel).toHaveTextContent('prompt:b1:fathers-and-sons');
    expect(panel).toHaveTextContent('verdict:b1:w1');
  });
});

describe('BookDetailView tabs', () => {
  const twoAdaptations = {
    items: [adaptationItem, { ...adaptationItem, id: 'w2', title: 'Сериал', kind: 'series' }],
    total: 2,
  };

  it('orders the tabs and selects the adaptations for a book that has them', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue(twoAdaptations);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((t) => t.textContent)).toEqual(['Экранизации (2)', 'Рецензии', 'Похожие книги']);
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel', { name: 'Экранизации (2)' })).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Экранизации' })).toHaveClass('sr-only');
  });

  it('has no adaptations tab and selects the reviews without adaptations', async () => {
    getBook.mockResolvedValue(book);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((t) => t.textContent)).toEqual(['Рецензии', 'Похожие книги']);
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByText(/экранизаци/i)).not.toBeInTheDocument();
  });

  it('opens the requested tab in the first render and keeps the adaptations in the page', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue(twoAdaptations);

    render(await BookDetailView({ slug: 'fathers-and-sons', tab: 'reviews' }));

    expect(screen.getByRole('tab', { name: 'Рецензии' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel', { name: 'Рецензии' })).toBeVisible();
    // Hidden, but present in the HTML.
    const adaptationsPanel = document.getElementById('book-tabpanel-adaptations');
    expect(adaptationsPanel).toHaveAttribute('hidden');
    expect(adaptationsPanel).toHaveTextContent('Отцы и дети (фильм)');
    expect(adaptationsPanel).toHaveTextContent('Сериал');
  });

  it('falls back to the default tab for an unknown value and a missing tab', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue(twoAdaptations);
    const first = render(await BookDetailView({ slug: 'fathers-and-sons', tab: 'bogus' }));
    expect(screen.getByRole('tab', { name: 'Экранизации (2)' })).toHaveAttribute('aria-selected', 'true');
    first.unmount();

    adaptationsForBook.mockResolvedValue({ items: [], total: 0 });
    render(await BookDetailView({ slug: 'fathers-and-sons', tab: 'adaptations' }));
    expect(screen.getByRole('tab', { name: 'Рецензии' })).toHaveAttribute('aria-selected', 'true');
  });

  it('hides the panel headings and gives the similar books an empty fallback', async () => {
    getBook.mockResolvedValue(book);

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByText('reviews-panel hidden-heading:yes')).toBeInTheDocument();
    expect(document.getElementById('book-tabpanel-similar')).toHaveTextContent('Похожих книг пока нет.');
  });

  it('shows the header link only with visible adaptations', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue(twoAdaptations);
    const first = render(await BookDetailView({ slug: 'fathers-and-sons' }));
    const link = screen.getByRole('link', { name: '2 экранизации' });
    expect(link).toHaveAttribute('href', '?tab=adaptations#book-tabs');
    first.unmount();

    adaptationsForBook.mockResolvedValue({ items: [], total: 0 });
    render(await BookDetailView({ slug: 'fathers-and-sons' }));
    expect(screen.queryByRole('link', { name: /экранизаци/ })).not.toBeInTheDocument();
  });

  it('selects the tab and scrolls it into view from the header link', async () => {
    getBook.mockResolvedValue(book);
    adaptationsForBook.mockResolvedValue(twoAdaptations);
    const scrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = scrollIntoView;

    render(await BookDetailView({ slug: 'fathers-and-sons', tab: 'reviews' }));
    expect(screen.getByRole('tab', { name: 'Рецензии' })).toHaveAttribute('aria-selected', 'true');

    fireEvent.click(screen.getByRole('link', { name: '2 экранизации' }));

    expect(screen.getByRole('tab', { name: 'Экранизации (2)' })).toHaveAttribute('aria-selected', 'true');
    expect(window.location.search).toBe('?tab=adaptations');
    expect(scrollIntoView).toHaveBeenCalled();
    expect(scrollIntoView.mock.contexts[0]).toHaveAttribute('id', 'book-tabs');
  });

  it('selects the reviews from the review count link of the rating summary', async () => {
    getBook.mockResolvedValue({ ...book, rating: 4.3, ratingsCount: 21, reviewsCount: 3 });
    adaptationsForBook.mockResolvedValue(twoAdaptations);
    window.HTMLElement.prototype.scrollIntoView = vi.fn();

    render(await BookDetailView({ slug: 'fathers-and-sons' }));
    fireEvent.click(screen.getByRole('link', { name: '3 рецензии' }));

    expect(screen.getByRole('tab', { name: 'Рецензии' })).toHaveAttribute('aria-selected', 'true');
    expect(window.location.search).toBe('?tab=reviews');
  });
});

describe('BookDetailView details and genres', () => {
  it('renders a definition list with the language and omits missing terms', async () => {
    getBook.mockResolvedValue(book);

    const { container } = render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(container.querySelector('dl')).not.toBeNull();
    expect(screen.getByText('Год издания').nextElementSibling).toHaveTextContent('1862');
    expect(screen.getByText('Язык').nextElementSibling).toHaveTextContent('Русский');
    expect(screen.queryByText('Страниц')).not.toBeInTheDocument();
    expect(screen.queryByText('ISBN')).not.toBeInTheDocument();
  });

  it('links genres to the filtered catalog', async () => {
    getBook.mockResolvedValue({ ...book, genres: [{ id: 'g1', name: 'Роман', slug: 'roman' }] });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.getByRole('link', { name: 'Роман' })).toHaveAttribute('href', '/books?genre_id=g1');
  });
});

describe('BookDetailView description behaviour', () => {
  it('keeps paragraphs and collapses a long description with Ещё / Свернуть', async () => {
    const long = 'Абзац один.\n\n' + 'Длинный текст. '.repeat(40);
    getBook.mockResolvedValue({ ...book, description: long });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const text = screen.getByText(/Абзац один\./);
    expect(text).toHaveClass('whitespace-pre-line', 'line-clamp-6');

    fireEvent.click(screen.getByRole('button', { name: 'Ещё' }));
    expect(text).not.toHaveClass('line-clamp-6');
    fireEvent.click(screen.getByRole('button', { name: 'Свернуть' }));
    expect(text).toHaveClass('line-clamp-6');
  });

  it('a short description has no toggle', async () => {
    getBook.mockResolvedValue({ ...book, description: 'Коротко.' });

    render(await BookDetailView({ slug: 'fathers-and-sons' }));

    expect(screen.queryByRole('button', { name: 'Ещё' })).not.toBeInTheDocument();
  });
});

describe('BookDetailView JSON-LD', () => {
  it('embeds a Book without aggregateRating when there are no ratings', async () => {
    getBook.mockResolvedValue(book);

    const { container } = render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const script = container.querySelector('script[type="application/ld+json"]') as HTMLElement;
    const ld = JSON.parse(script.textContent as string);
    expect(ld['@type']).toBe('Book');
    expect(ld).not.toHaveProperty('aggregateRating');
  });
});

describe('BookDetailView JSON-LD escaping', () => {
  it('never lets a title break out of the script tag', async () => {
    const title = '</script><img onerror=x>';
    getBook.mockResolvedValue({ ...book, title });

    const { container } = render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const script = container.querySelector('script[type="application/ld+json"]') as HTMLElement;
    expect(script.innerHTML).not.toContain('<');
    expect(JSON.parse(script.textContent as string).name).toBe(title);
  });
});

describe('BookDetailView actions column', () => {
  it('keeps the actions in the sticky aside, before the content column', async () => {
    getBook.mockResolvedValue(book);

    const { container } = render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const aside = container.querySelector('aside') as HTMLElement;
    expect(aside.querySelector('[data-testid="add-to-list"]')).toBeNull();
    expect(aside).toHaveClass('md:sticky');
  });

  it('renders the sections inside the content column, after the genres', async () => {
    getBook.mockResolvedValue({ ...book, genres: [{ id: 'g1', name: 'Роман', slug: 'roman' }] });

    const { container } = render(await BookDetailView({ slug: 'fathers-and-sons' }));

    const aside = container.querySelector('aside') as HTMLElement;
    const genre = screen.getByRole('link', { name: 'Роман' });
    const similar = screen.getByRole('heading', { name: 'Похожие книги', hidden: true });
    const reviews = screen.getByRole('heading', { name: 'Рецензии' });
    expect(aside.contains(similar)).toBe(false);
    expect(genre.compareDocumentPosition(reviews) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(reviews.compareDocumentPosition(similar) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
